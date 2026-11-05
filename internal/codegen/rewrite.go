package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// SetPackageName rewrites the name on the package clause of src to name. Only the
// clause is parsed, so src may be incomplete.
func SetPackageName(src []byte, name string) ([]byte, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "src.go", src, parser.PackageClauseOnly)
	if err != nil {
		return nil, err
	}

	if f.Name.Name == name {
		return src, nil
	}

	edit := textEdit{fset.Position(f.Name.Pos()).Offset, fset.Position(f.Name.End()).Offset, name}

	return applyEdits(src, []textEdit{edit}), nil
}

// RenameReceivers rewrites the type of method receivers in src. renames maps the
// name a receiver has to the name it gets. Nothing else changes: a parameter, field
// or declaration with the same name keeps it.
func RenameReceivers(src []byte, renames map[string]string) ([]byte, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "src.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var edits []textEdit

	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}

		for _, r := range fn.Recv.List {
			id := receiverIdent(r.Type)
			if id == nil {
				continue
			}

			if to, ok := renames[id.Name]; ok && to != id.Name {
				edits = append(edits, textEdit{fset.Position(id.Pos()).Offset, fset.Position(id.End()).Offset, to})
			}
		}
	}

	if len(edits) == 0 {
		return src, nil
	}

	return applyEdits(src, edits), nil
}
