package nerdgraphclient

import (
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

// optionalSchema has a query and mutations with optional arguments, among them one
// whose result is a scalar, so the mutation is rendered with a stand-in.
func optionalSchema() *schema.Schema {
	nonNull := func(r schema.TypeRef) schema.TypeRef {
		return schema.TypeRef{Kind: schema.KindNonNull, OfType: &r}
	}

	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "itemUpdate", Type: obj("Item"), Args: []schema.Field{
				{Name: "id", Type: nonNull(scalar("ID"))},
				{Name: "version", Type: scalar("Int")},
				{Name: "note", Type: scalar("String")},
			}},
			{Name: "itemTouch", Type: scalar("Boolean"), Args: []schema.Field{
				{Name: "version", Type: scalar("Int")},
			}},
			{Name: "itemPing", Type: nonNull(scalar("Boolean"))},
			{Name: "itemOld", Type: obj("Item"), Args: []schema.Field{
				{Name: "version", Type: scalar("Int")},
			}},
		}},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: obj("Actor")}}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "items", Type: listOf(obj("Item")), Args: []schema.Field{
					{Name: "first", Type: scalar("Int")},
					{Name: "cursor", Type: scalar("String")},
					{Name: "filter", Type: listOf(scalar("String"))},
					{Name: "id", Type: scalar("ID")},
					{Name: "name", Type: nonNull(scalar("String"))},
				}},
				{Name: "oldItems", Type: listOf(obj("Item")), Args: []schema.Field{
					{Name: "first", Type: scalar("Int")},
				}},
			}},
			{Name: "Item", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: scalar("String")}}},
			{Name: "ID", Kind: schema.KindScalar},
			{Name: "Int", Kind: schema.KindScalar},
			{Name: "Boolean", Kind: schema.KindScalar},
			{Name: "String", Kind: schema.KindScalar},
		},
	}
}

func optionalPackage(dir string, optional bool) *config.PackageConfig {
	return &config.PackageConfig{
		Name:       "items",
		Path:       dir,
		Generators: []string{"typegen", "nerdgraphclient"},
		Types:      []config.TypeConfig{{Name: "ID", FieldTypeOverride: "string"}, {Name: "Item"}},
		Queries: []config.Query{{Path: []string{"actor"}, Endpoints: []config.EndpointConfig{
			{Name: "items", MaxQueryFieldDepth: 1, IncludeOptionalArguments: optional, MethodName: "GetItemList", ContextMethodName: "GetItemListCtx"},
			{Name: "oldItems", MaxQueryFieldDepth: 1},
		}}},
		Mutations: []config.MutationConfig{
			{Name: "itemUpdate", MaxQueryFieldDepth: 1, IncludeOptionalArguments: optional},
			{Name: "itemTouch", IncludeOptionalArguments: optional},
			{Name: "itemPing"},
			{Name: "itemOld", MaxQueryFieldDepth: 1},
		},
	}
}

// contextTemplate prints, as newrelic-client-go's template does, a method that calls a
// WithContext variant, which builds the variables.
const contextTemplate = `package {{.GoPackageName}}
{{$clientType := .ClientTypeName}}
{{range .Mutations}}
func (a *{{$clientType}}) {{.Name | title}}(
  {{- range .Signature.Input}}
    {{.Name | untitle}} {{.Type}},
  {{- end}}
) (*{{ .Signature.Return | join ", "}}) {
  return a.{{.Name | title}}WithContext(context.Background(),
  {{- range .Signature.Input}}
    {{.Name | untitle}},
  {{- end}}
  )
}

func (a *{{$clientType}}) {{.Name | title}}WithContext(
  ctx context.Context,
  {{- range .Signature.Input}}
    {{.Name | untitle}} {{.Type}},
  {{- end}}
) (*{{ .Signature.Return | join ", "}}) {
	resp := {{.Name}}QueryResponse{}
	vars := map[string]interface{}{
  {{- range .QueryVars}}
    "{{.Key}}": {{.Value | untitle}},
  {{- end}}
	}

	if err := a.client.NerdGraphQueryWithContext(ctx, {{.Name}}Mutation, vars, &resp); err != nil {
		return nil, err
	}

	return &resp.{{first .Signature.Return}}, nil
}

const {{.Name}}Mutation = ` + "`{{ .QueryString }}`" + `
{{end}}
{{range .Queries}}
func (a *{{$clientType}}) {{.MethodName}}(
  {{- range .Signature.Input}}
    {{.Name | untitle}} {{.Type}},
  {{- end}}
) (*{{ .Signature.Return | join ", "}}) {
  return a.{{.ContextMethodName}}(context.Background(),
  {{- range .Signature.Input}}
    {{.Name | untitle}},
  {{- end}}
  )
}

func (a *{{$clientType}}) {{.ContextMethodName}}(
  ctx context.Context,
  {{- range .Signature.Input}}
    {{.Name | untitle}} {{.Type}},
  {{- end}}
) (*{{ .Signature.Return | join ", "}}) {
	resp := {{.ResponseObjectType}}{}
	vars := map[string]interface{}{
  {{- range .QueryVars}}
    "{{.Key}}": {{.Value | untitle}},
  {{- end}}
	}

	if err := a.client.NerdGraphQueryWithContext(ctx, {{.QueryConstName}}, vars, &resp); err != nil {
		return nil, err
	}

	return &resp.{{.Signature.ReturnPath | join "."}}.{{.FieldName}}, nil
}

const {{.QueryConstName}} = ` + "`{{ .QueryString }}`" + `
{{end}}
`

func contextTemplates(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "client.go.tmpl"), []byte(contextTemplate), 0o600))

	return dir
}

func TestGenerate_IncludeOptionalArguments(t *testing.T) {
	t.Parallel()

	templates := map[string]string{
		"own template":                    "../../templates/nerdgraphclient",
		"template with a context variant": contextTemplates(t),
	}

	for name, dir := range templates {
		dir := dir

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gc := genConfig()
			gc.TemplateDir = dir

			src, err := runSchema(t, optionalSchema(), optionalPackage(t.TempDir(), true), gc)
			require.NoError(t, err)

			formatted, err := format.Source([]byte(src))
			require.NoError(t, err)
			assert.Equal(t, string(formatted), src, "the output is gofmt-clean")

			fns := functions(t, src)

			// The method that builds the request, by the name the template gives it.
			list := fns["GetItemList"]
			if dir != "../../templates/nerdgraphclient" {
				list = fns["GetItemListCtx"]
				assert.NotContains(t, fns["GetItemList"], "delete(vars", "the method that only calls the variant has no variables")
			}

			assert.Contains(t, src, "first *int,\n\tcursor *string,\n\tfilter []string,\n\tiD *string,\n\tname string,")
			assert.Contains(t, list, "\t\t\"first\":  first,\n")
			assert.Contains(t, list, "}\n\n\tif first == nil {\n\t\tdelete(vars, \"first\")\n\t}\n\tif cursor == nil {\n\t\tdelete(vars, \"cursor\")\n\t}\n\tif filter == nil {\n\t\tdelete(vars, \"filter\")\n\t}\n\tif iD == nil {\n\t\tdelete(vars, \"id\")\n\t}\n\n\tif err := ")
			assert.NotContains(t, list, "name == nil", "a required argument is always sent")

			// Every optional argument is declared in the query.
			assert.Contains(t, src, "$first: Int,\n\t$cursor: String,\n\t$filter: [String],\n\t$id: ID,\n\t$name: String!,")

			update := fns["ItemUpdate"]
			if dir != "../../templates/nerdgraphclient" {
				update = fns["ItemUpdateWithContext"]
			}

			assert.Contains(t, update, "\"version\": version,")
			assert.Contains(t, update, "\n\tif version == nil {\n\t\tdelete(vars, \"version\")\n\t}\n\tif note == nil {\n\t\tdelete(vars, \"note\")\n\t}\n")
			assert.NotContains(t, update, "id == nil", "the required argument")

			// A mutation whose result is a scalar is rendered through a stand-in.
			touch := fns["ItemTouch"]
			if dir != "../../templates/nerdgraphclient" {
				touch = fns["ItemTouchWithContext"]
			}

			assert.Contains(t, touch, "if version == nil {\n\t\tdelete(vars, \"version\")")
			assert.NotContains(t, src, "tutoneResult")

			// Operations without the key are the code they always were.
			assert.NotContains(t, fns["ItemPing"], "delete(vars")
			assert.NotContains(t, fns["ItemOld"], "delete(vars")
			assert.NotContains(t, fns["GetOldItems"], "delete(vars")
			assert.Contains(t, src, "func (a *Items) GetOldItems(\n\tfirst int,")
		})
	}
}

func TestGenerate_WithoutIncludeOptionalArgumentsTheOutputIsWhatItAlwaysWas(t *testing.T) {
	t.Parallel()

	src, err := runSchema(t, optionalSchema(), optionalPackage(t.TempDir(), false), genConfig())
	require.NoError(t, err)

	assert.NotContains(t, src, "delete(vars")
	assert.NotContains(t, src, "*int")
	assert.Contains(t, src, "first int,\n\tcursor string,\n\tfilter []string,\n\tiD string,\n\tname string,")
	assert.NotContains(t, src, "$first", "the optional arguments are still not declared")
}
