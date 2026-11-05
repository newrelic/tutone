package nerdgraphclient

import (
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
