package nerdgraphclient

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/generators/typegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func obj(name string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindObject, Name: name} }
func scalar(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: n} }
func listOf(r schema.TypeRef) schema.TypeRef {
	return schema.TypeRef{Kind: schema.KindList, OfType: &r}
}

// testSchema has two query paths that both start at Actor and both reach a
// `linkedAccounts` field, plus a root field whose name differs from its type.
func testSchema() *schema.Schema {
	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "actor", Type: obj("Actor")},
				{Name: "docs", Type: obj("DocumentationFields")},
			}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "cloud", Type: obj("Cloud")},
				{Name: "account", Type: obj("Account"), Args: []schema.Field{
					{Name: "id", Type: schema.TypeRef{Kind: schema.KindNonNull, OfType: &schema.TypeRef{Kind: schema.KindScalar, Name: "Int"}}},
				}},
			}},
			{Name: "Account", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "cloud", Type: obj("AccountCloud")},
			}},
			{Name: "Cloud", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "linkedAccounts", Type: listOf(obj("LinkedAccount"))},
			}},
			{Name: "AccountCloud", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "linkedAccounts", Type: listOf(obj("LinkedAccount"))},
			}},
			{Name: "LinkedAccount", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "id", Type: scalar("Int")},
				{Name: "name", Type: scalar("String")},
			}},
			{Name: "DocumentationFields", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "items", Type: listOf(obj("LinkedAccount"))},
			}},
			{Name: "CollaborationThread", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "comment", Type: obj("CollaborationComment")},
				{Name: "title", Type: scalar("String")},
			}},
			{Name: "CollaborationComment", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "thread", Type: obj("CollaborationThread")},
				{Name: "body", Type: scalar("String")},
			}},
			{Name: "Int", Kind: schema.KindScalar},
			{Name: "String", Kind: schema.KindScalar},
		},
	}
}

func twoPathPackage(dir string, withMethodNames bool) *config.PackageConfig {
	e1 := config.EndpointConfig{Name: "linkedAccounts", MaxQueryFieldDepth: 2}
	e2 := config.EndpointConfig{Name: "linkedAccounts", MaxQueryFieldDepth: 2}

	if withMethodNames {
		e1.MethodName = "GetLinkedAccountsOrg"
		e2.MethodName = "GetLinkedAccountsForAccount"
	}

	return &config.PackageConfig{
		Name: "cloud",
		Path: dir,
		Types: []config.TypeConfig{
			{Name: "LinkedAccount"},
		},
		Queries: []config.Query{
			{Path: []string{"actor", "cloud"}, Endpoints: []config.EndpointConfig{e1}},
			{Path: []string{"actor", "account", "cloud"}, Endpoints: []config.EndpointConfig{e2}},
		},
	}
}

func genConfig() *config.GeneratorConfig {
	return &config.GeneratorConfig{
		Name:        "nerdgraphclient",
		FileName:    "{{.FileName}}_api.go",
		TemplateDir: "../../templates/nerdgraphclient",
	}
}

// run generates the package and returns the contents of the file it wrote.
func run(t *testing.T, pkg *config.PackageConfig, gc *config.GeneratorConfig) (string, error) {
	t.Helper()

	return runSchema(t, testSchema(), pkg, gc)
}

func runSchema(t *testing.T, s *schema.Schema, pkg *config.PackageConfig, gc *config.GeneratorConfig) (string, error) {
	t.Helper()

	g := &Generator{}

	if err := g.Generate(s, gc, pkg); err != nil {
		return "", err
	}

	if err := g.Execute(gc, pkg); err != nil {
		return "", err
	}

	b, err := os.ReadFile(filepath.Join(pkg.Path, pkg.GetFileName()+"_api.go"))
	require.NoError(t, err)

	return string(b), nil
}

// runTypes renders the package's types.go. The structs live there, not in the
// client file, so anything about struct declarations has to be checked here.
func runTypes(t *testing.T, pkg *config.PackageConfig) string {
	t.Helper()

	return runTypesIn(t, pkg, "../../templates/typegen")
}

func runTypesIn(t *testing.T, pkg *config.PackageConfig, templateDir string) string {
	t.Helper()

	return runTypesSchema(t, testSchema(), pkg, templateDir)
}

func runTypesSchema(t *testing.T, s *schema.Schema, pkg *config.PackageConfig, templateDir string) string {
	t.Helper()

	gc := &config.GeneratorConfig{Name: "typegen", FileName: "types.go", TemplateDir: templateDir}
	g := &typegen.Generator{}

	require.NoError(t, g.Generate(s, gc, pkg))
	require.NoError(t, g.Execute(gc, pkg))

	b, err := os.ReadFile(filepath.Join(pkg.Path, "types.go"))
	require.NoError(t, err)

	return string(b)
}

// structFields maps each struct in f to its field names and types, and counts how
// many times each struct name is declared.
func structFields(f *ast.File) (fields map[string]map[string]string, declared map[string]int) {
	fields = map[string]map[string]string{}
	declared = map[string]int{}

	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			declared[ts.Name.Name]++

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}

			fields[ts.Name.Name] = map[string]string{}

			for _, fld := range st.Fields.List {
				for _, n := range fld.Names {
					fields[ts.Name.Name][n.Name] = types.ExprString(fld.Type)
				}
			}
		}
	}

	return fields, declared
}

func parse(t *testing.T, src string) *ast.File {
	t.Helper()

	f, err := parser.ParseFile(token.NewFileSet(), "out.go", src, parser.ParseComments)
	require.NoError(t, err, "generated source must parse:\n%s", src)

	return f
}

func methodNames(f *ast.File) []string {
	var names []string

	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv != nil {
			names = append(names, fn.Name.Name)
		}
	}

	return names
}

func TestGenerate_SameEndpointNameUnderTwoPathsNeedsAMethodNameEach(t *testing.T) {
	t.Parallel()

	_, err := run(t, twoPathPackage(t.TempDir(), false), genConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetLinkedAccounts")
	assert.Contains(t, err.Error(), "method_name")
}

func TestGenerate_TwoPathsMergeIntoOneActorAndGetOneMethodEach(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src, err := run(t, twoPathPackage(dir, true), genConfig())
	require.NoError(t, err)

	f := parse(t, src)

	assert.ElementsMatch(t, []string{"GetLinkedAccountsOrg", "GetLinkedAccountsForAccount"}, methodNames(f))

	// The query constants are distinct too, or the package would not compile.
	assert.Equal(t, 1, strings.Count(src, "const getLinkedAccountsOrgQuery ="))
	assert.Equal(t, 1, strings.Count(src, "const getLinkedAccountsForAccountQuery ="))

	// Each method reads the field it was generated for, not its (renamed) method name.
	assert.Contains(t, src, "resp.Actor.Cloud.LinkedAccounts")
	assert.Contains(t, src, "resp.Actor.Account.Cloud.LinkedAccounts")

	// Both paths start at Actor. It is declared once and carries the field from each.
	fields, declared := structFields(parse(t, runTypes(t, twoPathPackage(dir, true))))

	for name, n := range declared {
		assert.Equal(t, 1, n, "type %s is declared %d times", name, n)
	}

	require.Contains(t, fields, "Actor")
	assert.Equal(t, "Cloud", fields["Actor"]["Cloud"])
	assert.Equal(t, "Account", fields["Actor"]["Account"])
}

func TestGenerate_PackageLineAndReceiverUseTheirOwnSettings(t *testing.T) {
	t.Parallel()

	pkg := twoPathPackage(t.TempDir(), true)
	pkg.Name = "organization"
	pkg.ClientTypeName = "OrganizationManagement"

	src, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	assert.Equal(t, "organization", parse(t, src).Name.Name)
	assert.Contains(t, src, "func (a *OrganizationManagement) GetLinkedAccountsOrg(")
	assert.NotContains(t, src, "func (a *Organization) ")
}

func TestGenerate_GoPackageNameDiffersFromTheConfigName(t *testing.T) {
	t.Parallel()

	pkg := twoPathPackage(t.TempDir(), true)
	pkg.Name = "scorecards"
	pkg.GoPackageName = "servicearchintelligence"

	src, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	assert.Equal(t, "servicearchintelligence", parse(t, src).Name.Name)
}

func TestGenerate_DefaultsKeepTheExistingOutput(t *testing.T) {
	t.Parallel()

	pkg := twoPathPackage(t.TempDir(), true)
	pkg.Name = "agentApplications"

	src, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	assert.Equal(t, "agentapplications", parse(t, src).Name.Name, "package line is lower-cased")
	assert.Contains(t, src, "func (a *AgentApplications) ", "receiver is the title-cased name")
}

func TestGenerate_RootResponseFieldIsNamedAfterTheFieldNotItsType(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{
		Name:  "agent",
		Path:  t.TempDir(),
		Types: []config.TypeConfig{{Name: "LinkedAccount"}},
		Queries: []config.Query{
			{Path: []string{"docs"}, Endpoints: []config.EndpointConfig{{Name: "items", MaxQueryFieldDepth: 1}}},
		},
	}

	src, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	// The method reads resp.Docs, so the response struct must have a Docs field,
	// even though the type behind it is called DocumentationFields.
	assert.Contains(t, src, "resp.Docs.Items")

	fields, _ := structFields(parse(t, runTypes(t, pkg)))

	require.Contains(t, fields, "itemsResponse")
	assert.Equal(t, map[string]string{"Docs": "DocumentationFields"}, fields["itemsResponse"])
}

func TestGenerate_RegeneratingKeepsHandWrittenMethodsAndIsStable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pkg := twoPathPackage(dir, true)
	file := filepath.Join(dir, "cloud_api.go")

	first, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	// Someone adds a method by hand to the generated file, and pins one generated method.
	edited := strings.Replace(first,
		"func (a *Cloud) GetLinkedAccountsOrg(",
		"// GetLinkedAccountsOrg has a hand fix.\n// DO NOT REGENERATE\nfunc (a *Cloud) GetLinkedAccountsOrg(",
		1)
	edited = strings.Replace(edited, "return &resp.Actor.Cloud.LinkedAccounts, nil", "return &resp.Actor.Cloud.LinkedAccounts, nil // hand fix", 1)
	edited += "\n// CloudAuthenticateIntegration was added by hand.\nfunc (a *Cloud) CloudAuthenticateIntegration() error { return nil }\n"

	require.NotEqual(t, first, edited)
	require.NoError(t, os.WriteFile(file, []byte(edited), 0o600))

	second, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	assert.Contains(t, second, "CloudAuthenticateIntegration", "an unmanaged method survives")
	assert.Contains(t, second, "// hand fix", "a pinned method keeps its edit")
	assert.Equal(t, 1, strings.Count(second, "func (a *Cloud) GetLinkedAccountsOrg("))
	assert.ElementsMatch(t,
		[]string{"GetLinkedAccountsOrg", "GetLinkedAccountsForAccount", "CloudAuthenticateIntegration"},
		methodNames(parse(t, second)))

	third, err := run(t, pkg, genConfig())
	require.NoError(t, err)
	assert.Equal(t, second, third, "regenerating again changes nothing")
}

func TestGenerate_PreserveCanBeTurnedOff(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pkg := twoPathPackage(dir, true)
	off := false
	pkg.PreserveHandWritten = &off

	first, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "cloud_api.go"),
		[]byte(first+"\nfunc (a *Cloud) ByHand() {}\n"), 0o600))

	second, err := run(t, pkg, genConfig())
	require.NoError(t, err)
	assert.NotContains(t, second, "ByHand")
}

func TestGenerate_CaseOnlyFileNameDifferenceReusesTheExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agentapplications_api.go"), []byte("package agentapplications\n"), 0o600))

	pkg := twoPathPackage(dir, true)
	pkg.Name = "agentApplications"

	// The long-standing spelling, {{.PackageName}}, asks for agentApplications_api.go.
	legacy := genConfig()
	legacy.FileName = "{{.PackageName}}_api.go"

	g := &Generator{}
	require.NoError(t, g.Generate(testSchema(), legacy, pkg))
	require.NoError(t, g.Execute(legacy, pkg), "existing configs keep working")

	var apiFiles []string

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), "_api.go") {
			apiFiles = append(apiFiles, e.Name())
		}
	}

	assert.Equal(t, []string{"agentapplications_api.go"}, apiFiles, "one file, so a case-sensitive file system builds")

	b, err := os.ReadFile(filepath.Join(dir, "agentapplications_api.go"))
	require.NoError(t, err)
	assert.Contains(t, string(b), "func (a *AgentApplications) GetLinkedAccountsOrg(")
}

// Two structs that contain each other by value are an invalid recursive type in
// Go. The generation path itself must break the cycle, not just a helper.
func TestGenerate_MutuallyRecursiveTypesCompileAsOnePointer(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{
		Name: "entities",
		Path: t.TempDir(),
		Types: []config.TypeConfig{
			{Name: "CollaborationThread"},
			{Name: "CollaborationComment"},
		},
	}

	fields, declared := structFields(parse(t, runTypes(t, pkg)))

	require.Equal(t, 1, declared["CollaborationThread"])
	require.Equal(t, 1, declared["CollaborationComment"])

	a := fields["CollaborationThread"]["Comment"]
	b := fields["CollaborationComment"]["Thread"]

	pointers := 0

	for _, ty := range []string{a, b} {
		if strings.HasPrefix(ty, "*") {
			pointers++
		}
	}

	assert.Equal(t, 1, pointers, "exactly one side of the cycle is a pointer; got %q and %q", a, b)
	assert.Equal(t, "string", fields["CollaborationThread"]["Title"], "other fields are untouched")
}

func TestGenerate_FieldOverrideWinsOverAutomaticRecursionBreaking(t *testing.T) {
	t.Parallel()

	yes := true
	pkg := &config.PackageConfig{
		Name: "entities",
		Path: t.TempDir(),
		Types: []config.TypeConfig{
			{Name: "CollaborationThread", Fields: []config.FieldConfig{{Name: "comment", Pointer: &yes}}},
			{Name: "CollaborationComment"},
		},
	}

	fields, _ := structFields(parse(t, runTypes(t, pkg)))

	assert.Equal(t, "*CollaborationComment", fields["CollaborationThread"]["Comment"])
	assert.Equal(t, "CollaborationThread", fields["CollaborationComment"]["Thread"], "the cycle is already broken, so the other side stays a value")
}

// legacyTemplates writes copies of the templates spelled the way
// newrelic-client-go's are, which cannot change: the lower-cased config name on the
// package line and its `title` on every receiver.
func legacyTemplates(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	legacy := func(file, templateDir string, swaps ...[2]string) {
		b, err := os.ReadFile(filepath.Join(templateDir, file))
		require.NoError(t, err)

		src := string(b)

		for _, sw := range swaps {
			require.Contains(t, src, sw[0], "%s no longer has the line this test makes legacy", file)
			src = strings.Replace(src, sw[0], sw[1], 1)
		}

		require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(src), 0o600))
	}

	pkgLine := [2]string{"package {{.GoPackageName}}", "package {{.PackageName | lower}}"}

	legacy("client.go.tmpl", "../../templates/nerdgraphclient", pkgLine,
		[2]string{"{{$clientType := .ClientTypeName}}", "{{$clientType := .PackageName | title}}"})
	legacy("types.go.tmpl", "../../templates/typegen", pkgLine)
	legacy("integration_test.go.tmpl", "../../templates/nerdgraphclient")

	return dir
}

func TestGenerate_TemplatesThatPredateTheNamingSettingsHonourThem(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		goPackage, clientType     string
		wantPackage, wantReceiver string
	}{
		"unset keeps what the template writes": {"", "", "organization", "Organization"},
		"client type only":                     {"", "OrganizationManagement", "organization", "OrganizationManagement"},
		"package only":                         {"servicearchintelligence", "", "servicearchintelligence", "Organization"},
		"both":                                 {"servicearchintelligence", "ServiceArchIntelligence", "servicearchintelligence", "ServiceArchIntelligence"},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			templates := legacyTemplates(t)

			pkg := twoPathPackage(dir, true)
			pkg.Name = "organization"
			pkg.GoPackageName = tc.goPackage
			pkg.ClientTypeName = tc.clientType
			pkg.IncludeIntegrationTest = true

			gc := genConfig()
			gc.TemplateDir = templates

			src, err := run(t, pkg, gc)
			require.NoError(t, err)

			// The client file, the types file and the integration test all agree.
			assert.Equal(t, tc.wantPackage, parse(t, src).Name.Name)
			assert.Equal(t, tc.wantPackage, parse(t, runTypesIn(t, pkg, templates)).Name.Name)

			integration, err := os.ReadFile(filepath.Join(dir, "organization_integration_test.go"))
			require.NoError(t, err)
			assert.Equal(t, tc.wantPackage, parse(t, string(integration)).Name.Name)

			// Every method has the configured receiver, and only receivers were touched.
			f := parse(t, src)
			require.NotEmpty(t, methodNames(f))

			for _, d := range f.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv != nil {
					assert.Equal(t, "*"+tc.wantReceiver, types.ExprString(fn.Recv.List[0].Type), fn.Name.Name)
				}
			}

			assert.Contains(t, src, "const getLinkedAccountsOrgQuery =")
		})
	}
}

func TestGenerate_LegacyReceiverRenameStillMatchesTheHandFixedMethod(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pkg := twoPathPackage(dir, true)
	pkg.Name = "organization"
	pkg.ClientTypeName = "OrganizationManagement"

	gc := genConfig()
	gc.TemplateDir = legacyTemplates(t)

	first, err := run(t, pkg, gc)
	require.NoError(t, err)

	// A generated method is pinned by hand under the configured receiver. The legacy
	// template writes another, so a rename after the merge would leave two.
	edited := strings.Replace(first,
		"func (a *OrganizationManagement) GetLinkedAccountsOrg(",
		"// DO NOT REGENERATE\nfunc (a *OrganizationManagement) GetLinkedAccountsOrg(", 1)
	edited = strings.Replace(edited, "return &resp.Actor.Cloud.LinkedAccounts, nil", "return &resp.Actor.Cloud.LinkedAccounts, nil // hand fix", 1)
	require.NotEqual(t, first, edited)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "organization_api.go"), []byte(edited), 0o600))

	second, err := run(t, pkg, gc)
	require.NoError(t, err)

	assert.Contains(t, second, "// hand fix")
	assert.ElementsMatch(t, []string{"GetLinkedAccountsOrg", "GetLinkedAccountsForAccount"}, methodNames(parse(t, second)))
}

func TestGenerate_KeepExistingEndpointsKeepTheHandWrittenMethods(t *testing.T) {
	t.Parallel()

	// Two endpoints that share a name under two paths, renamed by hand: the shape of
	// destinations in the notifications package.
	dir := t.TempDir()
	pkg := twoPathPackage(dir, false)

	_, err := run(t, pkg, genConfig())
	require.Error(t, err, "without keep_existing the shared name is a duplicate")

	for i := range pkg.Queries {
		pkg.Queries[i].Endpoints[0].KeepExisting = true
	}

	handWritten := `package cloud

import "context"

// GetLinkedAccountsOrg was renamed by hand.
func (a *Cloud) GetLinkedAccountsOrg(ctx context.Context) (*[]LinkedAccount, error) {
	resp := linkedAccountsResponse{}
	if err := a.client.NerdGraphQueryWithContext(ctx, getLinkedAccountsQueryOrg, nil, &resp); err != nil {
		return nil, err
	}

	return &resp.Actor.Cloud.LinkedAccounts, nil
}

const getLinkedAccountsQueryOrg = ` + "`query { actor { cloud { linkedAccounts { id } } } }`" + `

// GetLinkedAccountsForAccount was renamed by hand.
func (a *Cloud) GetLinkedAccountsForAccount(ctx context.Context) (*[]LinkedAccount, error) {
	resp := linkedAccountsResponse{}
	if err := a.client.NerdGraphQueryWithContext(ctx, getLinkedAccountsQueryAccount, nil, &resp); err != nil {
		return nil, err
	}

	return &resp.Actor.Account.Cloud.LinkedAccounts, nil
}

const getLinkedAccountsQueryAccount = ` + "`query { actor { account { cloud { linkedAccounts { id } } } } }`" + `
`
	file := filepath.Join(dir, "cloud_api.go")
	require.NoError(t, os.WriteFile(file, []byte(handWritten), 0o600))

	src, err := run(t, pkg, genConfig())
	require.NoError(t, err)

	f := parse(t, src)

	assert.ElementsMatch(t, []string{"GetLinkedAccountsOrg", "GetLinkedAccountsForAccount"}, methodNames(f), "nothing generated, nothing duplicated")
	assert.Equal(t, 1, strings.Count(src, "const getLinkedAccountsQueryOrg ="))
	assert.Equal(t, 1, strings.Count(src, "const getLinkedAccountsQueryAccount ="))
	assert.Contains(t, src, "// GetLinkedAccountsOrg was renamed by hand.")
	assert.Contains(t, src, "return &resp.Actor.Account.Cloud.LinkedAccounts, nil")

	again, err := run(t, pkg, genConfig())
	require.NoError(t, err)
	assert.Equal(t, src, again, "regenerating again changes nothing")

	// The hand-written methods read linkedAccountsResponse, which types.go still declares.
	fields, declared := structFields(parse(t, runTypes(t, pkg)))

	assert.Equal(t, 1, declared["linkedAccountsResponse"])
	assert.Equal(t, map[string]string{"Actor": "Actor"}, fields["linkedAccountsResponse"])
	assert.Equal(t, "Account", fields["Actor"]["Account"])
}

func TestGenerate_KeepExistingMutationIsNotGenerated(t *testing.T) {
	t.Parallel()

	mutationSchema := func() *schema.Schema {
		s := testSchema()
		s.MutationType.Fields = []schema.Field{
			{Name: "thingCreate", Type: obj("Thing"), Args: []schema.Field{{Name: "name", Type: scalar("String")}}},
			{Name: "thingDelete", Type: obj("Thing"), Args: []schema.Field{{Name: "id", Type: scalar("Int")}}},
		}
		s.Types = append(s.Types, &schema.Type{Name: "Thing", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: scalar("Int")}}})

		return s
	}

	handWritten := `package things

// ThingCreate was reshaped by hand.
func (a *Things) ThingCreate(name string, extra bool) (*Thing, error) { return nil, nil }

const ThingCreateMutation = "by hand"
`

	t.Run("the others are generated and the kept one survives", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "things_api.go"), []byte(handWritten), 0o600))

		pkg := &config.PackageConfig{Name: "things", Path: dir, Mutations: []config.MutationConfig{
			{Name: "thingCreate", MaxQueryFieldDepth: 1, KeepExisting: true},
			{Name: "thingDelete", MaxQueryFieldDepth: 1},
		}}

		src, err := runSchema(t, mutationSchema(), pkg, genConfig())
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{"ThingCreate", "ThingDelete"}, methodNames(parse(t, src)))
		assert.Contains(t, src, "func (a *Things) ThingCreate(name string, extra bool)", "the hand-written signature")
		assert.Equal(t, 1, strings.Count(src, "const ThingCreateMutation ="))
		assert.Contains(t, src, `"by hand"`)
		assert.Contains(t, src, "const ThingDeleteMutation =")
	})

	t.Run("a package that keeps all of its mutations still generates", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "things_api.go"), []byte(handWritten), 0o600))

		pkg := &config.PackageConfig{Name: "things", Path: dir, Mutations: []config.MutationConfig{
			{Name: "thingCreate", KeepExisting: true},
		}}

		src, err := runSchema(t, mutationSchema(), pkg, genConfig())
		require.NoError(t, err)

		assert.Equal(t, []string{"ThingCreate"}, methodNames(parse(t, src)))
		assert.Contains(t, src, `"by hand"`)
	})
}

func TestGenerate_KeepExistingTypeKeepsTheHandEditedDeclaration(t *testing.T) {
	t.Parallel()

	handEdit := func(t *testing.T, dir, generated string) {
		t.Helper()

		edited := strings.Replace(generated, "type LinkedAccount struct {", "type LinkedAccount struct {\n\tFromHand string", 1)
		require.NotEqual(t, generated, edited)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "types.go"), []byte(edited), 0o600))
	}

	t.Run("with keep_existing", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		pkg := twoPathPackage(dir, true)
		pkg.Types = []config.TypeConfig{{Name: "LinkedAccount", KeepExisting: true}}

		handEdit(t, dir, runTypes(t, pkg))

		again := runTypes(t, pkg)
		fields, declared := structFields(parse(t, again))

		assert.Equal(t, 1, declared["LinkedAccount"])
		assert.Equal(t, "string", fields["LinkedAccount"]["FromHand"], "the declaration is the existing one")
		assert.Contains(t, fields["LinkedAccount"], "Name", "and keeps its other fields")

		// The type is still expanded, so what refers to it is generated as before.
		assert.Equal(t, "[]LinkedAccount", fields["Cloud"]["LinkedAccounts"])

		assert.Equal(t, again, runTypes(t, pkg), "regenerating again changes nothing")
	})

	t.Run("without it the edit is regenerated away", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		pkg := twoPathPackage(dir, true)

		handEdit(t, dir, runTypes(t, pkg))

		fields, _ := structFields(parse(t, runTypes(t, pkg)))
		assert.NotContains(t, fields["LinkedAccount"], "FromHand")
	})
}

// companionSchema has what the typegen template declares beside a struct: an enum,
// an interface with an implementation, and a struct with a field of that interface,
// which gets an UnmarshalJSON.
func companionSchema() *schema.Schema {
	id := []schema.Field{{Name: "id", Type: scalar("String")}}
	item := schema.TypeRef{Kind: schema.KindInterface, Name: "Item"}

	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject},
			{Name: "Page", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "items", Type: listOf(item)},
				{Name: "color", Type: schema.TypeRef{Kind: schema.KindENUM, Name: "Color"}},
			}},
			{Name: "Item", Kind: schema.KindInterface, Fields: id, PossibleTypes: []schema.TypeRef{obj("Gadget")}},
			{Name: "Gadget", Kind: schema.KindObject, Fields: id, Interfaces: []schema.TypeRef{item}},
			{Name: "Color", Kind: schema.KindENUM, EnumValues: []schema.EnumValue{{Name: "BLUE"}, {Name: "RED"}}},
			{Name: "String", Kind: schema.KindScalar},
		},
	}
}

func TestGenerate_KeepExistingTypeAlsoKeepsWhatIsDeclaredBesideIt(t *testing.T) {
	t.Parallel()

	newPackage := func(dir string, keep bool) *config.PackageConfig {
		return &config.PackageConfig{
			Name: "pages",
			Path: dir,
			Types: []config.TypeConfig{
				{Name: "Page", KeepExisting: keep},
				{Name: "Item", KeepExisting: keep},
				{Name: "Color", KeepExisting: keep},
			},
		}
	}

	generate := func(t *testing.T, dir string, keep bool) string {
		t.Helper()

		return runTypesSchema(t, companionSchema(), newPackage(dir, keep), "../../templates/typegen")
	}

	// Edits to what the template declares beside each type, and one to a method of a
	// type that is not kept. Each puts its marker inside the declaration.
	edits := []struct{ from, to, marker, note string }{
		{
			"func (x *Page) UnmarshalJSON(b []byte) error {",
			"func (x *Page) UnmarshalJSON(b []byte) error {\n\t// hand edit: Page.UnmarshalJSON",
			"hand edit: Page.UnmarshalJSON", "method of a kept struct",
		},
		{
			"func (x *Item) ImplementsItem() {}",
			"func (x *Item) ImplementsItem() { /* hand edit: Item.ImplementsItem */ }",
			"hand edit: Item.ImplementsItem", "method of a kept interface",
		},
		{
			"func UnmarshalItemInterface(b []byte) (*ItemInterface, error) {",
			"func UnmarshalItemInterface(b []byte) (*ItemInterface, error) {\n\t// hand edit: UnmarshalItemInterface",
			"hand edit: UnmarshalItemInterface", "function of a kept interface",
		},
		{
			"type ItemInterface interface {",
			"type ItemInterface interface {\n\t// hand edit: ItemInterface",
			"hand edit: ItemInterface", "Go interface of a kept interface",
		},
		{
			"var ColorTypes = struct {",
			"var ColorTypes = struct {\n\t// hand edit: ColorTypes",
			"hand edit: ColorTypes", "variable of a kept enum",
		},
		{
			"func (x *Gadget) ImplementsItem() {}",
			"func (x *Gadget) ImplementsItem() { /* hand edit: Gadget.ImplementsItem */ }",
			"hand edit: Gadget.ImplementsItem", "method of a type that is not kept",
		},
	}

	handEdit := func(t *testing.T, dir, generated string) {
		t.Helper()

		edited := generated

		for _, e := range edits {
			require.Contains(t, edited, e.from, "the template no longer declares the %s", e.note)
			edited = strings.Replace(edited, e.from, e.to, 1)
		}

		require.NoError(t, os.WriteFile(filepath.Join(dir, "types.go"), []byte(edited), 0o600))
	}

	t.Run("with keep_existing", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		handEdit(t, dir, generate(t, dir, true))

		again := generate(t, dir, true)

		for _, e := range edits {
			if strings.Contains(e.note, "not kept") {
				assert.NotContains(t, again, e.marker, "%s: the generator's own", e.note)
				continue
			}

			assert.Equal(t, 1, strings.Count(again, e.marker), "%s: kept, once", e.note)
		}

		// One declaration of each, whatever was kept.
		f := parse(t, again)
		_, declared := structFields(f)

		for _, name := range []string{"Page", "Item", "Gadget", "Color", "ItemInterface"} {
			assert.Equal(t, 1, declared[name], name)
		}

		assert.Equal(t, 1, strings.Count(again, "func UnmarshalItemInterface("))
		assert.Equal(t, 1, strings.Count(again, "func (x *Page) UnmarshalJSON("))
		assert.Equal(t, 1, strings.Count(again, "var ColorTypes ="))

		assert.Equal(t, again, generate(t, dir, true), "regenerating again changes nothing")
	})

	t.Run("without it the edits are regenerated away", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		handEdit(t, dir, generate(t, dir, false))

		assert.NotContains(t, generate(t, dir, false), "hand edit")
	})
}

func unionOf(name string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindUnion, Name: name} }

// catalogSchema has a union that two root fields return, a list of it, and
// members that need no config to be reached.
func catalogSchema() *schema.Schema {
	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "actor", Type: obj("Actor")},
			}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "catalog", Type: obj("Catalog")},
			}},
			{Name: "Catalog", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "result", Type: unionOf("SearchResult")},
				{Name: "results", Type: listOf(unionOf("SearchResult"))},
			}},
			{
				Name: "SearchResult", Kind: schema.KindUnion,
				PossibleTypes: []schema.TypeRef{obj("Template"), obj("Quickstart")},
			},
			{Name: "Template", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "id", Type: scalar("Int")},
			}},
			{Name: "Quickstart", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "title", Type: scalar("String")},
			}},
			{Name: "Int", Kind: schema.KindScalar},
			{Name: "String", Kind: schema.KindScalar},
		},
	}
}

// returnsSchema has mutations that return a list of IDs, a Boolean, a custom scalar, an
// object with and without arguments, and an account-scoped endpoint that takes its own
// accountId.
func returnsSchema() *schema.Schema {
	nonNull := func(r schema.TypeRef) schema.TypeRef {
		return schema.TypeRef{Kind: schema.KindNonNull, OfType: &r}
	}

	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "thingAdd", Type: listOf(scalar("ID")), Args: []schema.Field{
				{Name: "ids", Type: nonNull(listOf(nonNull(scalar("ID"))))},
			}},
			{Name: "thingDelete", Type: nonNull(scalar("Boolean")), Args: []schema.Field{
				{Name: "accountId", Type: nonNull(scalar("Int"))},
			}},
			{Name: "thingPing", Type: nonNull(scalar("Boolean"))},
			{Name: "thingStamp", Type: scalar("EpochSeconds")},
			{Name: "thingRegister", Type: obj("Thing")},
			{Name: "thingCreate", Type: obj("Thing"), Args: []schema.Field{{Name: "name", Type: scalar("String")}}},
		}},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: obj("Actor")}}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "account", Type: obj("Account"), Args: []schema.Field{{Name: "id", Type: nonNull(scalar("Int"))}}},
				{Name: "notifications", Type: obj("Notifications")},
			}},
			{Name: "Account", Kind: schema.KindObject, Fields: []schema.Field{{Name: "cloud", Type: obj("Cloud")}}},
			{Name: "Cloud", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "details", Type: listOf(obj("Details")), Args: []schema.Field{
					{Name: "accountId", Type: nonNull(scalar("Int"))},
					{Name: "region", Type: nonNull(scalar("String"))},
				}},
			}},
			{Name: "Notifications", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "oAuthUrl", Type: obj("OAuthUrlResponse")},
				{Name: "health", Type: scalar("String")},
			}},
			{Name: "OAuthUrlResponse", Kind: schema.KindObject, Fields: []schema.Field{{Name: "url", Type: scalar("String")}}},
			{Name: "Details", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: scalar("String")}}},
			{Name: "Thing", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: scalar("String")}}},
			{Name: "ID", Kind: schema.KindScalar},
			{Name: "Int", Kind: schema.KindScalar},
			{Name: "Boolean", Kind: schema.KindScalar},
			{Name: "String", Kind: schema.KindScalar},
			{Name: "EpochSeconds", Kind: schema.KindScalar},
		},
	}
}

func catalogPackage(dir string) *config.PackageConfig {
	return &config.PackageConfig{
		Name: "catalog",
		Path: dir,
		Queries: []config.Query{{
			Path: []string{"actor", "catalog"},
			Endpoints: []config.EndpointConfig{
				{Name: "result", MaxQueryFieldDepth: 2},
				{Name: "results", MaxQueryFieldDepth: 2},
			},
		}},
	}
}

// stdlibStubs are the parts of the standard library the generated code uses, small
// enough to type-check in a millisecond where the real packages take seconds.
var stdlibStubs = map[string]string{
	"encoding/json": "package json\ntype RawMessage []byte\nfunc Unmarshal(data []byte, v any) error { return nil }\n",
	"fmt":           "package fmt\nfunc Errorf(format string, a ...any) error { return nil }\n",
}

// stubImporter imports the stubs above.
type stubImporter struct {
	fset *token.FileSet
}

func (i stubImporter) Import(path string) (*types.Package, error) {
	src, ok := stdlibStubs[path]
	if !ok {
		return nil, fmt.Errorf("the generated code imports %q, which the test has no stub for", path)
	}

	f, err := parser.ParseFile(i.fset, path+".go", src, 0)
	if err != nil {
		return nil, err
	}

	return (&types.Config{}).Check(path, i.fset, []*ast.File{f}, nil)
}

// typeCheck fails the test unless the package made of the sources compiles. It
// catches what parsing cannot: a method declared twice, a type that is not declared.
func typeCheck(t *testing.T, sources ...string) {
	t.Helper()

	fset := token.NewFileSet()

	var files []*ast.File

	for i, src := range sources {
		f, err := parser.ParseFile(fset, fmt.Sprintf("file%d.go", i), src, parser.ParseComments)
		require.NoError(t, err, "source must parse:\n%s", src)

		files = append(files, f)
	}

	_, err := (&types.Config{Importer: stubImporter{fset: fset}}).Check("catalog", fset, files, nil)
	require.NoError(t, err)
}

func TestGenerate_UnionIsGeneratedLikeAnInterfaceAndCompiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templates := legacyTemplates(t)
	src := runTypesSchema(t, catalogSchema(), catalogPackage(dir), filepath.Join(templates))

	typeCheck(t, src)

	fields, declared := structFields(parse(t, src))

	assert.Equal(t, 0, declared["SearchResult"], "a union has no struct")
	assert.Equal(t, 1, declared["SearchResultInterface"])
	assert.Equal(t, "SearchResultInterface", fields["Catalog"]["Result"])
	assert.Equal(t, "[]SearchResultInterface", fields["Catalog"]["Results"])

	assert.Contains(t, src, "func (x *Template) ImplementsSearchResult() {}")
	assert.Contains(t, src, "func (x *Quickstart) ImplementsSearchResult() {}")
	assert.Contains(t, src, "func UnmarshalSearchResultInterface(b []byte) (*SearchResultInterface, error)")
	assert.Contains(t, src, `case "Template":`)
	assert.Contains(t, src, `case "Quickstart":`)

	// Catalog is only on the way to the endpoints, and decodes the union all the same.
	assert.Contains(t, src, "func (x *Catalog) UnmarshalJSON(b []byte) error")
	assert.Equal(t, 2, strings.Count(src, "UnmarshalSearchResultInterface(*"), "one call for the single field, one for each element of the list")
}

func TestGenerate_UnionEndpointsReturnTheInterfaceAndSelectByTypename(t *testing.T) {
	t.Parallel()

	templates := legacyTemplates(t)

	gc := genConfig()
	gc.TemplateDir = filepath.Join(templates)

	src, err := runSchema(t, catalogSchema(), catalogPackage(t.TempDir()), gc)
	require.NoError(t, err)

	assert.Contains(t, src, "GetResult() (*SearchResultInterface, error)")
	assert.Contains(t, src, "GetResults() (*[]SearchResultInterface, error)")

	for _, selection := range []string{"__typename", "... on Template {", "... on Quickstart {"} {
		assert.Contains(t, src, selection)
	}
}

// catalogClient is the client type the generated methods hang off, which the package
// declares by hand, with the NerdGraph call they make.
const catalogClient = `package catalog

type CatalogClient struct{ client nerdGraphClient }

type nerdGraphClient interface {
	NerdGraphQuery(query string, vars map[string]interface{}, respBody interface{}) error
}
`

// catalogSchemaOf is catalogSchema with SearchResult of another kind.
func catalogSchemaOf(kind schema.Kind) *schema.Schema {
	s := catalogSchema()

	for _, t := range s.Types {
		switch t.Name {
		case "Catalog":
			t.Fields[0].Type = schema.TypeRef{Kind: kind, Name: "SearchResult"}
			t.Fields[1].Type = listOf(schema.TypeRef{Kind: kind, Name: "SearchResult"})
		case "SearchResult":
			t.Kind = kind
		}
	}

	return s
}

func TestGenerate_GetResultOfATypeThePackageSuppliesCompiles(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind        schema.Kind
		config      config.TypeConfig
		handWritten string
		wantType    string
	}{
		"union": {
			schema.KindUnion, config.TypeConfig{Name: "SearchResult", SkipTypeCreate: true},
			"type SearchResult struct{ Title string }", "SearchResult",
		},
		"union the package has as its interface only": {
			schema.KindUnion, config.TypeConfig{Name: "SearchResult", SkipTypeCreate: true, FieldTypeOverride: "SearchResultInterface"},
			"type SearchResultInterface interface{ ImplementsSearchResult() }", "SearchResultInterface",
		},
		"interface": {
			schema.KindInterface, config.TypeConfig{Name: "SearchResult", SkipTypeCreate: true},
			"type SearchResultInterface interface{ ImplementsSearchResult() }", "SearchResultInterface",
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pkg := catalogPackage(t.TempDir())
			pkg.ClientTypeName = "CatalogClient"
			pkg.Types = []config.TypeConfig{tc.config}
			pkg.Queries[0].Endpoints = pkg.Queries[0].Endpoints[:1]

			s := catalogSchemaOf(tc.kind)

			client, err := runSchema(t, s, pkg, genConfig())
			require.NoError(t, err)

			typesFile := runTypesSchema(t, s, pkg, "../../templates/typegen")

			fields, _ := structFields(parse(t, typesFile))
			assert.Equal(t, tc.wantType, fields["Catalog"]["Result"], "GetResult reads it")
			assert.Contains(t, functions(t, client)["GetResult"], "resp.Actor.Catalog.Result")
			assert.Contains(t, functions(t, client)["GetResult"], "(*"+tc.wantType+", error)")

			typeCheck(t, catalogClient+tc.handWritten, client, typesFile)
		})
	}
}

func TestGenerate_UnmarshalJSONDeclaredByHandIsDeclaredOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	handWritten := "package catalog\n\nimport \"encoding/json\"\n\nfunc (x *Catalog) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, &struct{}{}) }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "catalog_extra.go"), []byte(handWritten), 0o600))

	src := runTypesSchema(t, catalogSchema(), catalogPackage(dir), filepath.Join(legacyTemplates(t)))

	assert.NotContains(t, src, "func (x *Catalog) UnmarshalJSON")
	typeCheck(t, src, handWritten)
}

func TestGenerate_RegeneratingAUnionPackageChangesNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templates := filepath.Join(legacyTemplates(t))

	first := runTypesSchema(t, catalogSchema(), catalogPackage(dir), templates)
	second := runTypesSchema(t, catalogSchema(), catalogPackage(dir), templates)

	assert.Equal(t, first, second)
}

// A struct with an UnmarshalJSON decodes the fields the schema lists, so it does not
// decode a field that keep_removed_fields keeps. The field stays, and the struct
// compiles.
func TestGenerate_KeptRemovedFieldOfAStructWithAnUnmarshalJSONStaysAndCompiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templates := "../../templates/typegen"

	// The file as a schema that still had a legacy field generated it.
	older := runTypesSchema(t, catalogSchema(), catalogPackage(dir), templates)
	require.Contains(t, older, "type Catalog struct {")

	older = strings.Replace(older, "type Catalog struct {", "type Catalog struct {\n\tLegacy string `json:\"legacy,omitempty\"`", 1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "types.go"), []byte(older), 0o600))

	pkg := catalogPackage(dir)
	pkg.KeepRemovedFields = true

	kept := runTypesSchema(t, catalogSchema(), pkg, templates)

	fields, _ := structFields(parse(t, kept))
	assert.Equal(t, "string", fields["Catalog"]["Legacy"], "the field stays")
	assert.Contains(t, kept, "// Not in the NerdGraph schema; kept so existing code compiles.")

	assert.Contains(t, kept, "func (x *Catalog) UnmarshalJSON(b []byte) error", "the struct still decodes its interface fields")
	assert.Contains(t, kept, `case "result":`)
	typeCheck(t, kept)
}

func returnsPackage(dir string) *config.PackageConfig {
	return &config.PackageConfig{
		Name:       "things",
		Path:       dir,
		Generators: []string{"typegen", "nerdgraphclient"},
		Types: []config.TypeConfig{
			{Name: "ID", FieldTypeOverride: "string"},
			{Name: "EpochSeconds", FieldTypeOverride: "nrtime.EpochSeconds"},
			{Name: "Thing"},
			{Name: "OAuthUrlResponse"},
		},
		Mutations: []config.MutationConfig{{Name: "thing.*", MaxQueryFieldDepth: 1}},
		Queries: []config.Query{
			{Path: []string{"actor", "account", "cloud"}, Endpoints: []config.EndpointConfig{{Name: "details", MaxQueryFieldDepth: 1}}},
			{Path: []string{"actor", "notifications"}, Endpoints: []config.EndpointConfig{
				{Name: "oAuthUrl", MaxQueryFieldDepth: 1},
				{Name: "health", MaxQueryFieldDepth: 1},
			}},
		},
	}
}

// functions maps each function in f, by name, to its source in src.
func functions(t *testing.T, src string) map[string]string {
	t.Helper()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "out.go", src, parser.ParseComments)
	require.NoError(t, err, "generated source must parse:\n%s", src)

	out := map[string]string{}

	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			out[fn.Name.Name] = src[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset]
		}
	}

	return out
}

// undeclaredResponses lists the response structs the client file builds that neither file declares.
func undeclaredResponses(t *testing.T, client, typesFile string) []string {
	t.Helper()

	_, declaredInClient := structFields(parse(t, client))
	_, declaredInTypes := structFields(parse(t, typesFile))

	var missing []string

	ast.Inspect(parse(t, client), func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		if id, ok := lit.Type.(*ast.Ident); ok && declaredInClient[id.Name]+declaredInTypes[id.Name] == 0 {
			missing = append(missing, id.Name)
		}

		return true
	})

	return missing
}

func TestGenerate_MutationsReturningScalarsAndListsCompile(t *testing.T) {
	t.Parallel()

	// The second template set is spelled the way newrelic-client-go's is.
	templates := map[string]struct{ client, types string }{
		"own templates":    {"../../templates/nerdgraphclient", "../../templates/typegen"},
		"legacy templates": {legacyTemplates(t), legacyTemplates(t)},
	}

	for name, dirs := range templates {
		name, dirs := name, dirs

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pkg := returnsPackage(t.TempDir())
			gc := genConfig()
			gc.TemplateDir = dirs.client

			client, err := runSchema(t, returnsSchema(), pkg, gc)
			require.NoError(t, err)

			typesFile := runTypesSchema(t, returnsSchema(), pkg, dirs.types)
			fns := functions(t, client)

			// A list of IDs comes back as the list.
			assert.Contains(t, fns["ThingAdd"], "(*[]string, error)")
			assert.Contains(t, fns["ThingAdd"], "return &resp.Result, nil")
			assert.NotContains(t, client, "resp.[]")

			// A Boolean comes back as a bool, held in a field that can be decoded into.
			assert.Contains(t, fns["ThingDelete"], "(*bool, error)")
			assert.Contains(t, fns["ThingDelete"], "return &resp.Result, nil")

			// A type from another package is written as it is.
			assert.Contains(t, fns["ThingStamp"], "(*nrtime.EpochSeconds, error)")

			fields, _ := structFields(parse(t, client))
			assert.Equal(t, map[string]string{"Result": "[]string"}, fields["ThingAddQueryResponse"])
			assert.Equal(t, map[string]string{"Result": "bool"}, fields["ThingDeleteQueryResponse"])

			// A scalar has no selection set, with or without variables.
			assert.Contains(t, client, "const ThingAddMutation = `mutation(\n\t$ids: [ID!]!,\n) { thingAdd(\n\tids: $ids,\n) }`")
			assert.Contains(t, client, "const ThingPingMutation = `mutation { thingPing }`")

			assert.Empty(t, undeclaredResponses(t, client, typesFile))
			assert.NotContains(t, client+typesFile, "tutoneResult", "no stand-in is left in the output")
		})
	}
}

func TestGenerate_ArgumentlessMutationGetsItsResponseStruct(t *testing.T) {
	t.Parallel()

	pkg := returnsPackage(t.TempDir())

	client, err := runSchema(t, returnsSchema(), pkg, genConfig())
	require.NoError(t, err)

	typesFile := runTypesSchema(t, returnsSchema(), pkg, "../../templates/typegen")
	typeFields, declared := structFields(parse(t, typesFile))

	// The template declares the response only when the mutation has variables.
	assert.NotContains(t, client, "type ThingRegisterQueryResponse")
	assert.Equal(t, 1, declared["ThingRegisterQueryResponse"])
	assert.Equal(t, map[string]string{"Thing": "Thing"}, typeFields["ThingRegisterQueryResponse"])
	assert.Contains(t, functions(t, client)["ThingRegister"], "return &resp.Thing, nil")

	assert.Equal(t, 1, declared["ThingPingQueryResponse"])
	assert.Equal(t, map[string]string{"Result": "bool"}, typeFields["ThingPingQueryResponse"])
	assert.Equal(t, map[string]string{"Result": "nrtime.EpochSeconds"}, typeFields["ThingStampQueryResponse"])

	// One that has variables is declared by the template, and only there.
	assert.Equal(t, 0, declared["ThingCreateQueryResponse"])
	assert.Contains(t, client, "type ThingCreateQueryResponse struct")

	// Without empty parentheses the query is valid GraphQL.
	assert.Contains(t, client, "const ThingRegisterMutation = `mutation { thingRegister {\n\tid\n} }`")
	assert.NotContains(t, client, "mutation(\n)")
	assert.Empty(t, undeclaredResponses(t, client, typesFile))
}

func TestGenerate_ArgumentlessMutationReturningAUnionDecodesItAndCompiles(t *testing.T) {
	t.Parallel()

	s := catalogSchema()
	s.MutationType.Fields = []schema.Field{
		{Name: "catalogPick", Type: unionOf("SearchResult")},
		{Name: "catalogPickAll", Type: listOf(unionOf("SearchResult"))},
	}

	pkg := &config.PackageConfig{
		Name:           "catalog",
		Path:           t.TempDir(),
		ClientTypeName: "CatalogClient",
		Generators:     []string{"typegen", "nerdgraphclient"},
		Mutations:      []config.MutationConfig{{Name: "catalogPick.*", MaxQueryFieldDepth: 2}},
	}

	client, err := runSchema(t, s, pkg, genConfig())
	require.NoError(t, err)

	typesFile := runTypesSchema(t, s, pkg, "../../templates/typegen")

	// The result is held in the field the method reads and decoded by __typename, under the
	// key the response has it, which is the name of the mutation in the schema.
	assert.Contains(t, client, "return &resp.SearchResultInterface, nil")
	assert.Contains(t, client, "return &resp.Result, nil")

	for _, want := range []string{
		"func (x *CatalogPickQueryResponse) UnmarshalJSON(b []byte) error",
		`case "catalogPick":`,
		"x.SearchResultInterface = *xxx",
		"func (x *CatalogPickAllQueryResponse) UnmarshalJSON(b []byte) error",
		`case "catalogPickAll":`,
		"x.Result = append(x.Result, *xxx)",
	} {
		assert.Contains(t, typesFile, want)
	}

	assert.Empty(t, undeclaredResponses(t, client, typesFile))
	typeCheck(t, catalogClient, client, typesFile)
}

func TestGenerate_PackageWithoutTheClientGeneratorGetsNoMutationResponses(t *testing.T) {
	t.Parallel()

	pkg := returnsPackage(t.TempDir())
	pkg.Generators = []string{"typegen"}

	_, declared := structFields(parse(t, runTypesSchema(t, returnsSchema(), pkg, "../../templates/typegen")))

	assert.Zero(t, declared["ThingRegisterQueryResponse"])
	assert.Zero(t, declared["ThingPingQueryResponse"])
}

func TestGenerate_AccountScopedEndpointWithItsOwnAccountIdTakesOneAccountID(t *testing.T) {
	t.Parallel()

	client, err := runSchema(t, returnsSchema(), returnsPackage(t.TempDir()), genConfig())
	require.NoError(t, err)

	fn := functions(t, client)["GetDetails"]

	assert.Equal(t, 1, strings.Count(fn, "accountID int"), "the parameter is declared once")
	assert.Contains(t, fn, "region string")
	assert.Regexp(t, `"accountID":\s+accountID,`, fn)
	assert.Regexp(t, `"accountId":\s+accountID,`, fn, "the argument gets the same value")

	assert.Contains(t, client, "$accountID: Int!,\n\t$accountId: Int!,")
	assert.Contains(t, client, "account(id: $accountID)")
	assert.Contains(t, client, "accountId: $accountId,")
}

func TestGenerate_ResponseStructHasAFieldForATypeWhoseGoNameDiffersFromItsSchemaName(t *testing.T) {
	t.Parallel()

	pkg := returnsPackage(t.TempDir())

	fields, _ := structFields(parse(t, runTypesSchema(t, returnsSchema(), pkg, "../../templates/typegen")))

	// The method reads resp.Actor.Notifications.OAuthURL.
	assert.Equal(t, "OAuthURLResponse", fields["Notifications"]["OAuthURL"])
}

func TestGenerate_ScalarEndpointHasNoSelection(t *testing.T) {
	t.Parallel()

	client, err := runSchema(t, returnsSchema(), returnsPackage(t.TempDir()), genConfig())
	require.NoError(t, err)

	assert.Contains(t, client, "const getHealthQuery = `query { actor { notifications { health } } }`")
}
