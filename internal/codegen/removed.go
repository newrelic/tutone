package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/util"
)

// keptRemovedNote is the line a struct field or enum value gets when it is kept
// although the schema no longer lists it, in its own paragraph of the doc comment.
// It is not a "Deprecated:" paragraph: a schema that lacks the field may hide it from
// the key that generated the file rather than have removed it, and a deprecation
// would make linters flag every caller.
const keptRemovedNote = "// Not in the NerdGraph schema; kept so existing code compiles."

// KeepRemovedFields adds to generated the struct fields and enum values that
// existing declares and generated lacks, so code that uses them still compiles
// after the schema dropped them.
//
// Only types that both sources declare are looked at, so nothing the existing file
// lacks is ever added. A struct field is matched to the generated one by its json
// name: a field the schema still lists is never added a second time, and when its
// Go type changed the generated declaration stands. An enum value is matched by
// name. A kept field or value is copied as the existing file has it, with a
// note added to its doc comment, and is placed where the generator
// would have sorted it: by Go name, among the generated ones.
//
// A field whose Go name the generated struct already uses for another json name is
// not kept, because both could not compile. skip maps a Go type name to the Go
// names of the fields the package config skips: they are left out on purpose, so
// they are not kept either.
//
// The UnmarshalJSON that a struct is generated with decodes the fields the schema
// lists, so it does not decode a kept field. The queries the generator writes never
// select a field the schema lacks, so a response does not carry one.
//
// Imports the kept declarations need are carried over; the caller runs goimports
// after, which drops any that turn out to be unused. Both inputs must be valid Go.
// The kept members are returned as Type.Member.
func KeepRemovedFields(generated, existing []byte, skip map[string][]string) ([]byte, []string, error) {
	fset := token.NewFileSet()

	newFile, err := parser.ParseFile(fset, "generated.go", generated, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing generated source: %w", err)
	}

	oldFile, err := parser.ParseFile(fset, "existing.go", existing, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("existing file does not parse, refusing to overwrite it: %w", err)
	}

	k := removedKeeper{fset: fset, existing: existing, skip: skip, generatedComments: newFile.Comments}

	oldStructs := structTypes(oldFile)
	oldTables := enumTables(oldFile)

	newStructs := structTypes(newFile)
	for _, name := range slices.Sorted(maps.Keys(newStructs)) {
		if old, ok := oldStructs[name]; ok {
			k.keepStructFields(name, newStructs[name], old)
		}
	}

	newTables := enumTables(newFile)
	for _, name := range slices.Sorted(maps.Keys(newTables)) {
		if old, ok := oldTables[name]; ok {
			k.keepEnumValues(name, newTables[name], old, oldFile.Comments)
		}
	}

	if len(k.kept) == 0 {
		return generated, nil, nil
	}

	out := addMissingImports(applyEdits(generated, k.edits), oldFile)

	return out, k.kept, nil
}

// removedKeeper collects the edits that put removed members into the generated source.
type removedKeeper struct {
	fset     *token.FileSet
	existing []byte
	skip     map[string][]string

	// generatedComments are the comments of the generated file. The syntax tree does
	// not attach a comment to the entries of a literal, so they are looked up here.
	generatedComments []*ast.CommentGroup

	edits []textEdit
	kept  []string
}

// member is an entry of a struct's field list or of an enum table's literal, as
// the text it occupies in its source, doc comment included.
type member struct {
	name       string
	start, end int
	// text is what a kept member is copied from, note included.
	text string
	// jsonName is the name a struct field has in JSON, empty if it has none.
	jsonName string
}

// enumTable is the variable the typegen template declares for an enum: a literal of
// an anonymous struct type, whose fields are the values and whose entries set each.
type enumTable struct {
	fields  *ast.StructType
	literal *ast.CompositeLit
}

func (k *removedKeeper) offset(p token.Pos) int {
	return k.fset.Position(p).Offset
}

func (k *removedKeeper) keepStructFields(typeName string, generated, existing *ast.StructType) {
	present := k.generatedMembers(generated)

	jsonNames := map[string]bool{}
	goNames := map[string]bool{}

	for _, m := range present {
		jsonNames[m.jsonName] = true
		goNames[m.name] = true
	}

	var kept []member

	for _, f := range existing.Fields.List {
		m, ok := k.existingField(f)
		if !ok || m.jsonName == "" || jsonNames[m.jsonName] || util.StringInStrings(m.name, k.skip[typeName]) {
			continue
		}

		if goNames[m.name] {
			log.WithFields(log.Fields{
				"type":  typeName,
				"field": m.name,
				"json":  m.jsonName,
			}).Warn("not keeping a field that is not in the schema: the generated struct has a field of the same Go name")

			continue
		}

		kept = append(kept, m)
		k.kept = append(k.kept, typeName+"."+m.name)
	}

	k.insert(present, kept, k.offset(generated.Fields.Opening)+1, "")
}

func (k *removedKeeper) keepEnumValues(tableName string, generated, existing enumTable, existingComments []*ast.CommentGroup) {
	present := k.generatedMembers(generated.fields)

	have := map[string]bool{}
	for _, m := range present {
		have[m.name] = true
	}

	literals := map[string]string{}
	for _, m := range k.literalMembers(existing.literal, existingComments) {
		literals[m.name] = string(k.existing[m.start:m.end])
	}

	var fields, values []member

	for _, f := range existing.fields.Fields.List {
		m, ok := k.existingField(f)
		if !ok || have[m.name] || literals[m.name] == "" {
			continue
		}

		fields = append(fields, m)
		values = append(values, member{name: m.name, text: literals[m.name]})
		k.kept = append(k.kept, tableName+"."+m.name)
	}

	k.insert(present, fields, k.offset(generated.fields.Fields.Opening)+1, "")
	k.insert(k.literalMembers(generated.literal, k.generatedComments), values, k.offset(generated.literal.Lbrace)+1, ",")
}

// generatedMembers lists the fields of a generated struct, in source order.
func (k *removedKeeper) generatedMembers(s *ast.StructType) []member {
	var members []member

	for _, f := range s.Fields.List {
		if len(f.Names) == 0 {
			continue
		}

		start := k.offset(f.Pos())
		if f.Doc != nil {
			start = k.offset(f.Doc.Pos())
		}

		members = append(members, member{
			name:     f.Names[0].Name,
			start:    start,
			end:      k.fieldEnd(f),
			jsonName: jsonName(f),
		})
	}

	return members
}

// literalMembers lists the key: value entries of an enum table's literal. An entry
// starts at the comment on the lines just above it, which is its description.
func (k *removedKeeper) literalMembers(lit *ast.CompositeLit, comments []*ast.CommentGroup) []member {
	var members []member

	previousLine := k.fset.Position(lit.Lbrace).Line

	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		start := k.offset(kv.Pos())
		if c := k.commentAbove(comments, kv.Pos(), previousLine); c != nil {
			start = k.offset(c.Pos())
		}

		members = append(members, member{name: key.Name, start: start, end: k.offset(kv.End())})
		previousLine = k.fset.Position(kv.End()).Line
	}

	return members
}

// commentAbove returns the comment group that ends on the line before pos and
// starts after afterLine, so a comment trailing the previous entry is not taken.
func (k *removedKeeper) commentAbove(comments []*ast.CommentGroup, pos token.Pos, afterLine int) *ast.CommentGroup {
	line := k.fset.Position(pos).Line

	for _, c := range comments {
		if k.fset.Position(c.End()).Line == line-1 && k.fset.Position(c.Pos()).Line > afterLine {
			return c
		}
	}

	return nil
}

// fieldEnd is the offset where a field ends, its trailing comment included.
func (k *removedKeeper) fieldEnd(f *ast.Field) int {
	if f.Comment != nil {
		return k.offset(f.Comment.End())
	}

	return k.offset(f.End())
}

// existingField describes a field of the existing file as something to copy. Fields
// that are embedded, or that declare several names at once, cannot be matched to
// the schema and are left out.
func (k *removedKeeper) existingField(f *ast.Field) (member, bool) {
	if len(f.Names) != 1 {
		return member{}, false
	}

	text := string(k.existing[k.offset(f.Pos()):k.fieldEnd(f)])

	return member{
		name:     f.Names[0].Name,
		jsonName: jsonName(f),
		text:     k.withKeptNote(f.Doc, text),
	}, true
}

// withKeptNote puts the note after the doc comment of a field,
// or makes it the doc comment. A comment that already has it is left as it is, so
// regenerating a file that holds a kept field does not stack the paragraph.
func (k *removedKeeper) withKeptNote(doc *ast.CommentGroup, field string) string {
	if doc == nil {
		return keptRemovedNote + "\n\t" + field
	}

	docText := string(k.existing[k.offset(doc.Pos()):k.offset(doc.End())])
	if strings.Contains(docText, keptRemovedNote) {
		return docText + "\n\t" + field
	}

	return docText + "\n\t//\n\t" + keptRemovedNote + "\n\t" + field
}

// insert records the edits that put the kept members into the present ones, each
// ahead of the first present member that sorts after it, as the generator sorts
// by name, or at the end. open is the offset just inside the opening brace, for a
// list with no present member. sep is what ends an entry: "," in a literal.
func (k *removedKeeper) insert(present, kept []member, open int, sep string) {
	if len(kept) == 0 {
		return
	}

	sort.SliceStable(kept, func(i, j int) bool { return kept[i].name < kept[j].name })

	byOffset := map[int]string{}

	var offsets []int

	add := func(offset int, text string) {
		if _, ok := byOffset[offset]; !ok {
			offsets = append(offsets, offset)
		}

		byOffset[offset] += text
	}

	for _, m := range kept {
		next := sort.Search(len(present), func(i int) bool { return present[i].name > m.name })

		switch {
		case next < len(present):
			add(present[next].start, m.text+sep+"\n\t")
		case len(present) > 0:
			add(present[len(present)-1].end, sep+"\n\t"+m.text)
		default:
			add(open, "\n\t"+m.text+sep)
		}
	}

	for _, offset := range offsets {
		text := byOffset[offset]
		if len(present) == 0 {
			text += "\n"
		}

		k.edits = append(k.edits, textEdit{offset, offset, text})
	}
}

// structTypes maps the name of each struct type the file declares to its type.
func structTypes(file *ast.File) map[string]*ast.StructType {
	out := map[string]*ast.StructType{}

	for _, d := range file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}

		for _, spec := range gen.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				if st, ok := ts.Type.(*ast.StructType); ok {
					out[ts.Name.Name] = st
				}
			}
		}
	}

	return out
}

// enumTables maps the name of each variable the file declares as an anonymous
// struct with a literal, which is how the typegen template lists an enum's values.
func enumTables(file *ast.File) map[string]enumTable {
	out := map[string]enumTable{}

	for _, d := range file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}

		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}

			literal, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}

			if fields, ok := literal.Type.(*ast.StructType); ok {
				out[vs.Names[0].Name] = enumTable{fields: fields, literal: literal}
			}
		}
	}

	return out
}

// jsonName is the name in a field's json tag, or empty if it has no such tag or
// the tag leaves the field out of JSON.
func jsonName(f *ast.Field) string {
	if f.Tag == nil {
		return ""
	}

	tag, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return ""
	}

	name, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ",")
	if name == "-" {
		return ""
	}

	return name
}

// NoteKeptTypes gives the note a kept field gets to each whole type that merged carries
// over from the existing file although generated, what the generator wrote, no longer
// declares it, so a type the schema dropped is marked like a field it dropped.
//
// A removed generated type is recognised by its doc comment, whose first line is the
// form the template writes, `// <Name> - <description>`, where <Name> is the type's name
// or, for an interface, the name without its Interface suffix. Anything else, such as
// hand-written code with another doc form or none, is left alone, as is a declaration
// that carries DO NOT DELETE or DO NOT REGENERATE or whose name is in keep: those are
// kept as they are written. A doc that has the note already is left as it is, so
// regenerating does not stack it. The note follows the doc as its own paragraph.
//
// The names of the types noted are returned.
func NoteKeptTypes(merged, generated []byte, keep []string) ([]byte, []string, error) {
	fset := token.NewFileSet()

	mergedFile, err := parser.ParseFile(fset, "merged.go", merged, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing merged source: %w", err)
	}

	generatedFile, err := parser.ParseFile(fset, "generated.go", generated, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing generated source: %w", err)
	}

	produced := map[string]bool{}

	for _, name := range typeNames(generatedFile) {
		produced[name] = true
	}

	var (
		edits []textEdit
		noted []string
	)

	for _, d := range mergedFile.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}

		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || produced[ts.Name.Name] || util.StringInStrings(ts.Name.Name, keep) {
				continue
			}

			doc := ts.Doc
			if doc == nil && len(gen.Specs) == 1 {
				doc = gen.Doc
			}

			if doc == nil || !isGeneratedDoc(doc, ts.Name.Name) {
				continue
			}

			text := string(merged[fset.Position(doc.Pos()).Offset:fset.Position(gen.End()).Offset])
			if strings.Contains(text, keptRemovedNote) || hasMarker(text) {
				continue
			}

			end := fset.Position(doc.End()).Offset
			edits = append(edits, textEdit{end, end, "\n//\n" + keptRemovedNote})
			noted = append(noted, ts.Name.Name)
		}
	}

	if len(edits) == 0 {
		return merged, nil, nil
	}

	return applyEdits(merged, edits), noted, nil
}

// typeNames lists the types the file declares.
func typeNames(file *ast.File) []string {
	var names []string

	for _, d := range file.Decls {
		if gen, ok := d.(*ast.GenDecl); ok && gen.Tok == token.TYPE {
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					names = append(names, ts.Name.Name)
				}
			}
		}
	}

	return names
}

// isGeneratedDoc reports whether the first line of doc is `// <name> - ...`, the form
// the typegen template writes from a schema description, with the type's own name or
// the name an interface type is made from.
func isGeneratedDoc(doc *ast.CommentGroup, typeName string) bool {
	first := doc.List[0].Text

	return strings.HasPrefix(first, "// "+typeName+" - ") ||
		(strings.HasSuffix(typeName, "Interface") && strings.HasPrefix(first, "// "+strings.TrimSuffix(typeName, "Interface")+" - "))
}

// hasMarker reports whether text carries a marker that pins a declaration.
func hasMarker(text string) bool {
	upper := strings.ToUpper(text)

	for _, m := range markers {
		if strings.Contains(upper, m) {
			return true
		}
	}

	return false
}
