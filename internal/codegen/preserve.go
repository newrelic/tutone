package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// Markers that pin a declaration in a generated file. A declaration whose doc
// comment contains either one is kept exactly as it is in the existing file,
// and the generator's own version of it, if any, is dropped.
const (
	MarkerDoNotDelete     = "DO NOT DELETE"
	MarkerDoNotRegenerate = "DO NOT REGENERATE"
)

// decl is one top-level declaration located in a source file.
type decl struct {
	// names are the identifiers it declares, namespaced so a func, a method and a
	// package-level identifier cannot be confused.
	names      []string
	start, end int
	text       string
	marked     bool
}

// MergeHandWritten carries hand-written top-level declarations from the existing
// version of a generated file into the freshly generated one.
//
// A declaration from the existing file is kept when
//   - the generator does not produce any identifier it declares, or
//   - its doc comment contains DO NOT DELETE or DO NOT REGENERATE. Then it also
//     replaces the generator's declaration of the same name, in place.
//
// Anything else is the generator's own output and is replaced by the new version.
// Imports the kept code needs are carried over; the caller runs goimports after,
// which drops any that turn out to be unused.
//
// Both inputs must be valid Go. The names of what was carried over are returned.
func MergeHandWritten(generated, existing []byte) ([]byte, []string, error) {
	fset := token.NewFileSet()

	newFile, err := parser.ParseFile(fset, "generated.go", generated, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing generated source: %w", err)
	}

	oldFile, err := parser.ParseFile(fset, "existing.go", existing, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("existing file does not parse, refusing to overwrite it: %w", err)
	}

	newDecls := collectDecls(fset, newFile, generated)
	oldDecls := collectDecls(fset, oldFile, existing)

	generatedNames := map[string]int{}

	for i, d := range newDecls {
		for _, n := range d.names {
			generatedNames[n] = i
		}
	}

	var (
		edits    []textEdit
		appended []string
		kept     []string
		consumed = map[int]bool{}
	)

	for _, od := range oldDecls {
		twin, hasTwin := -1, false

		for _, n := range od.names {
			if i, ok := generatedNames[n]; ok {
				twin, hasTwin = i, true
				break
			}
		}

		switch {
		case od.marked && hasTwin && !consumed[twin]:
			// Pinned: the existing text wins, in the generator's position.
			consumed[twin] = true
			edits = append(edits, textEdit{newDecls[twin].start, newDecls[twin].end, od.text})
			kept = append(kept, od.names...)
		case od.marked, !hasTwin:
			// Pinned with nothing to replace, or not something the generator makes.
			appended = append(appended, od.text)
			kept = append(kept, od.names...)
		}
	}

	out := applyEdits(generated, edits)

	if len(appended) > 0 || len(edits) > 0 {
		out = addMissingImports(out, oldFile)
	}

	if len(appended) > 0 {
		out = append(out, []byte("\n\n"+strings.Join(appended, "\n\n")+"\n")...)
	}

	sort.Strings(kept)

	return out, displayNames(kept), nil
}

type textEdit struct {
	start, end int
	text       string
}

// applyEdits applies non-overlapping edits to src, last first so offsets hold.
func applyEdits(src []byte, edits []textEdit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })

	out := append([]byte(nil), src...)

	for _, e := range edits {
		out = append(out[:e.start:e.start], append([]byte(e.text), out[e.end:]...)...)
	}

	return out
}

// collectDecls lists the top-level declarations of file, other than imports.
func collectDecls(fset *token.FileSet, file *ast.File, src []byte) []decl {
	var decls []decl

	for _, d := range file.Decls {
		var (
			names  []string
			doc    *ast.CommentGroup
			marked bool
		)

		switch v := d.(type) {
		case *ast.FuncDecl:
			doc = v.Doc
			names = []string{funcKey(v)}
		case *ast.GenDecl:
			if v.Tok == token.IMPORT {
				continue
			}

			doc = v.Doc

			for _, spec := range v.Specs {
				names = append(names, specNames(spec)...)
			}
		default:
			continue
		}

		start := fset.Position(d.Pos()).Offset
		if doc != nil {
			start = fset.Position(doc.Pos()).Offset
		}

		end := fset.Position(d.End()).Offset

		if doc != nil {
			text := strings.ToUpper(doc.Text())
			marked = strings.Contains(text, MarkerDoNotDelete) || strings.Contains(text, MarkerDoNotRegenerate)
		}

		decls = append(decls, decl{
			names:  names,
			start:  start,
			end:    end,
			text:   string(src[start:end]),
			marked: marked,
		})
	}

	return decls
}

// funcKey names a function or method. Methods are keyed by receiver type, so
// (*T).Get and (*U).Get are different declarations.
func funcKey(f *ast.FuncDecl) string {
	if f.Recv == nil || len(f.Recv.List) == 0 {
		return "func:" + f.Name.Name
	}

	return "method:" + receiverName(f.Recv.List[0].Type) + "." + f.Name.Name
}

func receiverName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.StarExpr:
		return receiverName(v.X)
	case *ast.IndexExpr:
		return receiverName(v.X)
	case *ast.IndexListExpr:
		return receiverName(v.X)
	case *ast.Ident:
		return v.Name
	}

	return "?"
}

// specNames returns the package-level identifiers a spec declares. Types,
// consts and vars share one namespace in Go, so they share one key space here.
func specNames(spec ast.Spec) []string {
	var names []string

	switch s := spec.(type) {
	case *ast.TypeSpec:
		names = append(names, "id:"+s.Name.Name)
	case *ast.ValueSpec:
		for _, n := range s.Names {
			if n.Name != "_" {
				names = append(names, "id:"+n.Name)
			}
		}
	}

	return names
}

func displayNames(keys []string) []string {
	out := make([]string, 0, len(keys))

	for _, k := range keys {
		out = append(out, k[strings.Index(k, ":")+1:])
	}

	return out
}

// addMissingImports adds, to src, the imports of old that src lacks. src gets a
// second import declaration right after its package clause, which is legal Go,
// and goimports later merges and prunes it.
func addMissingImports(src []byte, old *ast.File) []byte {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "src.go", src, parser.ImportsOnly)
	if err != nil {
		return src
	}

	have := map[string]bool{}

	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		have[p] = true
	}

	var lines []string

	for _, imp := range old.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if have[p] {
			continue
		}

		line := "\t" + imp.Path.Value
		if imp.Name != nil {
			line = "\t" + imp.Name.Name + " " + imp.Path.Value
		}

		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return src
	}

	insertAt := fset.Position(f.Name.End()).Offset
	block := "\n\nimport (\n" + strings.Join(lines, "\n") + "\n)\n"

	return append(append(append([]byte(nil), src[:insertAt]...), []byte(block)...), src[insertAt:]...)
}
