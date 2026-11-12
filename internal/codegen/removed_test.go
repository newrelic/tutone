package codegen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/imports"
)

const generatedWidget = `package demo

type Widget struct {
	// The name.
	Name string ` + "`json:\"name,omitempty\"`" + `
	Size int ` + "`json:\"size,omitempty\"`" + `
}
`

func keepRemoved(t *testing.T, generated, existing string, skip map[string][]string) (string, []string) {
	t.Helper()

	out, kept, err := KeepRemovedFields([]byte(generated), []byte(existing), skip)
	require.NoError(t, err)

	formatted, err := imports.Process("out.go", out, nil)
	require.NoError(t, err, "output must format:\n%s", out)

	_, parseErr := parser.ParseFile(token.NewFileSet(), "out.go", formatted, parser.ParseComments)
	require.NoError(t, parseErr)

	return string(formatted), kept
}

func TestKeepRemovedFields_KeepsAFieldTheSchemaNoLongerLists(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	// The color.
	Color string ` + "`json:\"color,omitempty\"`" + `
	// The name.
	Name string ` + "`json:\"name,omitempty\"`" + `
	Size int ` + "`json:\"size,omitempty\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	assert.Equal(t, `package demo

type Widget struct {
	// The color.
	//
	// Not in the NerdGraph schema; kept so existing code compiles.
	Color string `+"`json:\"color,omitempty\"`"+`
	// The name.
	Name string `+"`json:\"name,omitempty\"`"+`
	Size int    `+"`json:\"size,omitempty\"`"+`
}
`, out)
	assert.Equal(t, []string{"Widget.Color"}, kept)
}

func TestKeepRemovedFields_FieldWithoutADocCommentGetsTheNote(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	Name string ` + "`json:\"name,omitempty\"`" + `
	Size int ` + "`json:\"size,omitempty\"`" + `
	Zeta bool ` + "`json:\"zeta\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	assert.Contains(t, out, "\t// Not in the NerdGraph schema; kept so existing code compiles.\n\tZeta bool `json:\"zeta\"`\n}")
	assert.Equal(t, []string{"Widget.Zeta"}, kept)
}

func TestKeepRemovedFields_PlacesKeptFieldsInTheGeneratorsOrder(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	Zeta  bool   ` + "`json:\"zeta\"`" + `
	Alpha string ` + "`json:\"alpha\"`" + `
	Name  string ` + "`json:\"name,omitempty\"`" + `
	Mid   string ` + "`json:\"mid\"`" + `
	Size  int    ` + "`json:\"size,omitempty\"`" + `
	Omega bool   ` + "`json:\"omega\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	fields := fieldNames(t, out, "Widget")
	assert.Equal(t, []string{"Alpha", "Mid", "Name", "Omega", "Size", "Zeta"}, fields, "sorted by Go name like the generated fields")
	assert.ElementsMatch(t, []string{"Widget.Alpha", "Widget.Mid", "Widget.Omega", "Widget.Zeta"}, kept)
}

func TestKeepRemovedFields_KeepsEveryFieldOfAnEmptyGeneratedStruct(t *testing.T) {
	t.Parallel()

	generated := "package demo\n\ntype Widget struct{}\n"
	existing := "package demo\n\ntype Widget struct {\n\tB int `json:\"b\"`\n\tA int `json:\"a\"`\n}\n"

	out, kept := keepRemoved(t, generated, existing, nil)

	assert.Equal(t, []string{"A", "B"}, fieldNames(t, out, "Widget"))
	assert.ElementsMatch(t, []string{"Widget.A", "Widget.B"}, kept)
}

func TestKeepRemovedFields_AFieldStillInTheSchemaIsNotDuplicated(t *testing.T) {
	t.Parallel()

	// The existing file spells the Go name and the type differently, but the json
	// name is the same field, so the generated declaration is the only one.
	existing := `package demo

type Widget struct {
	Title string ` + "`json:\"name,omitempty\"`" + `
	Size  *int   ` + "`json:\"size,omitempty\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	assert.Equal(t, generatedWidgetFormatted(), out)
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_AFieldWhoseTypeChangedIsRegeneratedAsTheSchemaSays(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	Name string ` + "`json:\"name,omitempty\"`" + `
	Size string ` + "`json:\"size,omitempty\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	assert.Contains(t, out, "Size int")
	assert.NotContains(t, out, "Size string")
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_AddsNothingTheExistingFileLacks(t *testing.T) {
	t.Parallel()

	generated := generatedWidget + "\ntype Brand struct {\n\tID string `json:\"id\"`\n}\n"
	existing := "package demo\n\ntype Widget struct {\n\tName string `json:\"name,omitempty\"`\n}\n"

	out, kept := keepRemoved(t, generated, existing, nil)

	assert.Equal(t, generatedWithBrand(), out, "a type, field or value only the generator has stays as generated")
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_OnlyTypesBothSourcesDeclareAreLookedAt(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Gone struct {
	Old string ` + "`json:\"old\"`" + `
}
` + generatedWidget[len("package demo\n"):]

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	assert.NotContains(t, out, "Gone")
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_SkippedFieldsAreNotKept(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	Color string ` + "`json:\"color\"`" + `
	Name  string ` + "`json:\"name,omitempty\"`" + `
	Shape string ` + "`json:\"shape\"`" + `
	Size  int    ` + "`json:\"size,omitempty\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, map[string][]string{"Widget": {"Color"}, "Other": {"Shape"}})

	assert.Equal(t, []string{"Name", "Shape", "Size"}, fieldNames(t, out, "Widget"), "skip_fields of another type does not apply")
	assert.Equal(t, []string{"Widget.Shape"}, kept)
}

func TestKeepRemovedFields_AFieldWhoseGoNameIsTakenIsNotKept(t *testing.T) {
	t.Parallel()

	// The schema now has a different field called Size. Keeping the old one too
	// would declare the name twice.
	existing := `package demo

type Widget struct {
	Name string ` + "`json:\"name,omitempty\"`" + `
	Size string ` + "`json:\"size_old\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	assert.Equal(t, []string{"Name", "Size"}, fieldNames(t, out, "Widget"))
	assert.Contains(t, out, "Size int")
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_FieldsThatCannotBeMatchedToTheSchemaAreLeftOut(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	Embedded
	A, B   int
	Hidden string ` + "`json:\"-\"`" + `
	Plain  string
	Name   string ` + "`json:\"name,omitempty\"`" + `
	Size   int    ` + "`json:\"size,omitempty\"`" + `
}
`

	out, kept := keepRemoved(t, generatedWidget, existing, nil)

	assert.Equal(t, generatedWidgetFormatted(), out)
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_KeepsATrailingCommentAndCopiesTheTagsAsTheyAre(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	Name string ` + "`json:\"name,omitempty\"`" + `
	Old  *string ` + "`json:\"old,omitempty\" yaml:\"old\"`" + ` // trailing
	Size int ` + "`json:\"size,omitempty\"`" + `
}
`

	out, _ := keepRemoved(t, generatedWidget, existing, nil)

	assert.Contains(t, out, "Old  *string `json:\"old,omitempty\" yaml:\"old\"` // trailing")
}

func TestKeepRemovedFields_AFieldAfterTheLastGeneratedOneStaysBelowItsTrailingComment(t *testing.T) {
	t.Parallel()

	generated := "package demo\n\ntype Widget struct {\n\tName string `json:\"name\"` // the name\n}\n"
	existing := "package demo\n\ntype Widget struct {\n\tName string `json:\"name\"`\n\tZeta bool `json:\"zeta\"`\n}\n"

	out, _ := keepRemoved(t, generated, existing, nil)

	assert.Contains(t, out, "Name string `json:\"name\"` // the name\n")
	assert.Equal(t, []string{"Name", "Zeta"}, fieldNames(t, out, "Widget"))
}

func TestKeepRemovedFields_RegeneratingTwiceIsStable(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Widget struct {
	// The color.
	Color string ` + "`json:\"color,omitempty\"`" + `
	Name  string ` + "`json:\"name,omitempty\"`" + `
	Size  int    ` + "`json:\"size,omitempty\"`" + `
}
`

	first, _ := keepRemoved(t, generatedWidget, existing, nil)
	second, kept := keepRemoved(t, generatedWidget, first, nil)

	assert.Equal(t, first, second)
	assert.Equal(t, []string{"Widget.Color"}, kept, "the field is still not in the schema")
	assert.Equal(t, 1, countOf(second, keptRemovedNote), "the note is not stacked")
}

func TestKeepRemovedFields_CarriesTheImportsTheKeptFieldNeeds(t *testing.T) {
	t.Parallel()

	existing := `package demo

import "github.com/newrelic/newrelic-client-go/v2/pkg/nrtime"

type Widget struct {
	Name string ` + "`json:\"name,omitempty\"`" + `
	Old  nrtime.EpochMilliseconds ` + "`json:\"old\"`" + `
	Size int ` + "`json:\"size,omitempty\"`" + `
}
`

	out, _ := keepRemoved(t, generatedWidget, existing, nil)

	assert.Contains(t, out, `"github.com/newrelic/newrelic-client-go/v2/pkg/nrtime"`)
	assert.Contains(t, out, "Old  nrtime.EpochMilliseconds")
}

const generatedEnum = `package demo

type Color string

var ColorTypes = struct {
	// Blue.
	BLUE Color
	// Red.
	RED Color
}{
	// Blue.
	BLUE: "BLUE",
	// Red.
	RED: "RED",
}
`

func TestKeepRemovedFields_KeepsAnEnumValueTheSchemaNoLongerLists(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Color string

var ColorTypes = struct {
	AMBER Color
	// Blue.
	BLUE Color
	// Red.
	RED Color
	TEAL Color
}{
	AMBER: "AMBER",
	// Blue.
	BLUE: "BLUE",
	// Red.
	RED: "RED",
	TEAL: "TEAL",
}
`

	out, kept := keepRemoved(t, generatedEnum, existing, nil)

	assert.Equal(t, `package demo

type Color string

var ColorTypes = struct {
	// Not in the NerdGraph schema; kept so existing code compiles.
	AMBER Color
	// Blue.
	BLUE Color
	// Red.
	RED Color
	// Not in the NerdGraph schema; kept so existing code compiles.
	TEAL Color
}{
	AMBER: "AMBER",
	// Blue.
	BLUE: "BLUE",
	// Red.
	RED:  "RED",
	TEAL: "TEAL",
}
`, out)
	assert.Equal(t, []string{"ColorTypes.AMBER", "ColorTypes.TEAL"}, kept)
}

func TestKeepRemovedFields_AnEnumValueStillInTheSchemaIsNotDuplicated(t *testing.T) {
	t.Parallel()

	out, kept := keepRemoved(t, generatedEnum, generatedEnum, nil)

	assert.Equal(t, generatedEnum, out)
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_AnEnumValueWithoutALiteralEntryIsNotKept(t *testing.T) {
	t.Parallel()

	existing := `package demo

type Color string

var ColorTypes = struct {
	BLUE Color
	OLD  Color
	RED  Color
}{
	BLUE: "BLUE",
	RED:  "RED",
}
`

	out, kept := keepRemoved(t, generatedEnum, existing, nil)

	assert.NotContains(t, out, "OLD")
	assert.Empty(t, kept)
}

func TestKeepRemovedFields_ATrailingCommentOnAnEntryIsNotTakenAsTheNextOnesDescription(t *testing.T) {
	t.Parallel()

	generated := `package demo

type Color string

var ColorTypes = struct {
	BLUE Color
	RED  Color
}{
	BLUE: "BLUE", // the sky
	RED:  "RED",
}
`
	existing := `package demo

type Color string

var ColorTypes = struct {
	BLUE  Color
	GREEN Color
	RED   Color
}{
	BLUE:  "BLUE",
	GREEN: "GREEN",
	RED:   "RED",
}
`

	out, _ := keepRemoved(t, generated, existing, nil)

	assert.Contains(t, out, "BLUE:  \"BLUE\", // the sky\n\tGREEN: \"GREEN\",\n\tRED:   \"RED\",")
}

func TestKeepRemovedFields_KeepsAllValuesOfAnEnumWithNone(t *testing.T) {
	t.Parallel()

	generated := "package demo\n\ntype Color string\n\nvar ColorTypes = struct{}{}\n"

	out, kept := keepRemoved(t, generated, generatedEnum, nil)

	assert.Equal(t, 2, countOf(out, keptRemovedNote))
	assert.Contains(t, out, `BLUE: "BLUE"`)
	assert.Contains(t, out, `RED: "RED"`)
	assert.Contains(t, out, "// Blue.\n\t//\n\t"+keptRemovedNote, "the description is kept and the note follows it")
	assert.Equal(t, []string{"ColorTypes.BLUE", "ColorTypes.RED"}, kept)
}

func TestKeepRemovedFields_UnparsableSourcesAreErrors(t *testing.T) {
	t.Parallel()

	_, _, err := KeepRemovedFields([]byte("package demo\n\nfunc broken( {"), []byte(generatedWidget), nil)
	require.ErrorContains(t, err, "generated source")

	_, _, err = KeepRemovedFields([]byte(generatedWidget), []byte("package demo\n\nfunc broken( {"), nil)
	require.ErrorContains(t, err, "refusing to overwrite")
}

func TestWriteFile_KeepsRemovedFieldsOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	const tmpl = "package demo\n\ntype Widget struct {\n\tName string `json:\"name\"`\n}\n"
	const existing = "package demo\n\ntype Widget struct {\n\tName string `json:\"name\"`\n\tOld  string `json:\"old\"`\n}\n"

	write := func(t *testing.T, keep, preserve bool) string {
		t.Helper()

		tmplDir, tmplName := writeTemplate(t, tmpl)
		out := t.TempDir()
		dest := filepath.Join(out, "types.go")
		require.NoError(t, os.WriteFile(dest, []byte(existing), 0o600))

		c := CodeGen{
			TemplateDir:         tmplDir,
			TemplateName:        tmplName,
			DestinationDir:      out,
			DestinationFile:     dest,
			PreserveHandWritten: preserve,
			KeepRemovedFields:   keep,
		}
		require.NoError(t, c.WriteFile(fakeGenerator{}))

		got, err := os.ReadFile(dest)
		require.NoError(t, err)

		return string(got)
	}

	assert.Contains(t, write(t, true, true), "Old string")
	assert.NotContains(t, write(t, false, true), "Old", "off is the output the generator always made")
	assert.NotContains(t, write(t, true, false), "Old", "without preserve_hand_written the file is replaced")
}

func TestWriteFile_KeepRemovedFieldsAlongsideAPinnedDeclaration(t *testing.T) {
	t.Parallel()

	const tmpl = "package demo\n\ntype Widget struct {\n\tName string `json:\"name\"`\n}\n\ntype Pinned struct {\n\tName string `json:\"name\"`\n}\n"
	const existing = `package demo

type Widget struct {
	Name string ` + "`json:\"name\"`" + `
	Old  string ` + "`json:\"old\"`" + `
}

// DO NOT REGENERATE
type Pinned struct {
	Name string ` + "`json:\"name\"`" + `
	Mine string ` + "`json:\"mine\"`" + `
}
`

	tmplDir, tmplName := writeTemplate(t, tmpl)
	out := t.TempDir()
	dest := filepath.Join(out, "types.go")
	require.NoError(t, os.WriteFile(dest, []byte(existing), 0o600))

	c := CodeGen{
		TemplateDir:         tmplDir,
		TemplateName:        tmplName,
		DestinationDir:      out,
		DestinationFile:     dest,
		PreserveHandWritten: true,
		KeepRemovedFields:   true,
	}
	require.NoError(t, c.WriteFile(fakeGenerator{}))

	got, err := os.ReadFile(dest)
	require.NoError(t, err)

	assert.Equal(t, 1, countOf(string(got), keptRemovedNote), "Widget.Old only; the pinned type is kept as written")
	assert.Equal(t, []string{"Name", "Old"}, fieldNames(t, string(got), "Widget"))
	assert.Equal(t, []string{"Name", "Mine"}, fieldNames(t, string(got), "Pinned"))
}

func generatedWidgetFormatted() string {
	formatted, err := imports.Process("out.go", []byte(generatedWidget), nil)
	if err != nil {
		panic(err)
	}

	return string(formatted)
}

func generatedWithBrand() string {
	formatted, err := imports.Process("out.go", []byte(generatedWidget+"\ntype Brand struct {\n\tID string `json:\"id\"`\n}\n"), nil)
	if err != nil {
		panic(err)
	}

	return string(formatted)
}

func countOf(s, sub string) int {
	return strings.Count(s, sub)
}

// fieldNames lists the names of the fields of the struct type typeName in src, in order.
func fieldNames(t *testing.T, src, typeName string) []string {
	t.Helper()

	f, err := parser.ParseFile(token.NewFileSet(), "src.go", src, 0)
	require.NoError(t, err)

	st, ok := structTypes(f)[typeName]
	require.True(t, ok, "no struct %s in:\n%s", typeName, src)

	var names []string

	for _, field := range st.Fields.List {
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
	}

	return names
}

const removedTypeFile = `package demo

// Widget - A widget.
type Widget struct {
	Name string ` + "`json:\"name\"`" + `
}
`

func noteKept(t *testing.T, generated, existing string, keep ...string) (string, []string) {
	t.Helper()

	merged, _, err := MergeHandWritten([]byte(generated), []byte(existing), keep...)
	require.NoError(t, err)

	out, noted, err := NoteKeptTypes(merged, []byte(generated), keep)
	require.NoError(t, err)

	formatted, err := imports.Process("out.go", out, nil)
	require.NoError(t, err, "output must format:\n%s", out)

	return string(formatted), noted
}

func TestNoteKeptTypes_AWholeRemovedTypeGetsTheNote(t *testing.T) {
	t.Parallel()

	existing := removedTypeFile + `
// EvaluationParameter - Parameters that can be used in custom evaluations
type EvaluationParameter struct {
	Name string ` + "`json:\"name\"`" + `
}

// EvaluationParameter - the second paragraph is not the first line.
func (x *EvaluationParameter) Describe() string { return "" }
`

	out, noted := noteKept(t, removedTypeFile, existing)

	assert.Equal(t, []string{"EvaluationParameter"}, noted)
	assert.Contains(t, out, "// EvaluationParameter - Parameters that can be used in custom evaluations\n//\n"+keptRemovedNote+"\ntype EvaluationParameter struct")
	assert.Equal(t, 1, countOf(out, keptRemovedNote), "the method and the generated type get none")
	assert.Contains(t, out, "// Widget - A widget.\ntype Widget struct")
}

func TestNoteKeptTypes_ADocWithSeveralParagraphsGetsTheNoteLast(t *testing.T) {
	t.Parallel()

	existing := removedTypeFile + `
// Old - First line.
//
// More about it.
type Old string
`

	out, noted := noteKept(t, removedTypeFile, existing)

	assert.Equal(t, []string{"Old"}, noted)
	assert.Contains(t, out, "// More about it.\n//\n"+keptRemovedNote+"\ntype Old string")
}

func TestNoteKeptTypes_AnInterfaceTypeIsRecognisedByItsDescription(t *testing.T) {
	t.Parallel()

	existing := removedTypeFile + `
// Gone - The interface.
type GoneInterface interface {
	ImplementsGone()
}
`

	out, noted := noteKept(t, removedTypeFile, existing)

	assert.Equal(t, []string{"GoneInterface"}, noted)
	assert.Contains(t, out, keptRemovedNote+"\ntype GoneInterface interface")
}

func TestNoteKeptTypes_HandWrittenAndPinnedDeclarationsAreUnchanged(t *testing.T) {
	t.Parallel()

	existing := removedTypeFile + `
// Helper is written by hand.
type Helper struct{}

type NoDoc struct{}

// Marked - Looks generated but is pinned.
//
// DO NOT DELETE
type Marked struct{}

// Named - Looks generated but the config keeps it.
type Named struct{}

// Elsewhere - Not the type's own name.
type Other struct{}
`

	out, noted := noteKept(t, removedTypeFile, existing, "Named")

	assert.Empty(t, noted)
	assert.NotContains(t, out, keptRemovedNote)
	assert.Contains(t, out, "type Helper struct{}")
	assert.Contains(t, out, "type NoDoc struct{}")
}

func TestNoteKeptTypes_ATypeTheGeneratorStillMakesIsLeftToKeepRemovedFields(t *testing.T) {
	t.Parallel()

	_, noted := noteKept(t, removedTypeFile, removedTypeFile)

	assert.Empty(t, noted)
}

func TestNoteKeptTypes_RegeneratingDoesNotStackTheNote(t *testing.T) {
	t.Parallel()

	existing := removedTypeFile + `
// Old - First line.
type Old string
`

	first, noted := noteKept(t, removedTypeFile, existing)
	require.Equal(t, []string{"Old"}, noted)

	second, noted := noteKept(t, removedTypeFile, first)

	assert.Empty(t, noted)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, countOf(second, keptRemovedNote))
}

func TestNoteKeptTypes_UnparsableSourceIsAnError(t *testing.T) {
	t.Parallel()

	_, _, err := NoteKeptTypes([]byte("package demo\nfunc ("), []byte(generatedWidget), nil)
	require.Error(t, err)
}

func TestWriteFile_AKeptTypeGetsTheNoteOnlyWithKeepRemovedFields(t *testing.T) {
	t.Parallel()

	const tmpl = "package demo\n\n// Widget - A widget.\ntype Widget struct {\n\tName string `json:\"name\"`\n}\n"
	const existing = tmpl + "\n// Old - Dropped from the schema.\ntype Old struct {\n\tName string `json:\"name\"`\n}\n"

	write := func(t *testing.T, keep bool) string {
		t.Helper()

		tmplDir, tmplName := writeTemplate(t, tmpl)
		out := t.TempDir()
		dest := filepath.Join(out, "types.go")
		require.NoError(t, os.WriteFile(dest, []byte(existing), 0o600))

		c := CodeGen{
			TemplateDir:         tmplDir,
			TemplateName:        tmplName,
			DestinationDir:      out,
			DestinationFile:     dest,
			PreserveHandWritten: true,
			KeepRemovedFields:   keep,
		}
		require.NoError(t, c.WriteFile(fakeGenerator{}))

		got, err := os.ReadFile(dest)
		require.NoError(t, err)

		return string(got)
	}

	assert.Contains(t, write(t, true), "// Old - Dropped from the schema.\n//\n"+keptRemovedNote+"\ntype Old struct")
	assert.NotContains(t, write(t, false), keptRemovedNote)
}
