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

	g := &Generator{}

	if err := g.Generate(testSchema(), gc, pkg); err != nil {
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

	gc := &config.GeneratorConfig{Name: "typegen", FileName: "types.go", TemplateDir: "../../templates/typegen"}
	g := &typegen.Generator{}

	require.NoError(t, g.Generate(testSchema(), gc, pkg))
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
