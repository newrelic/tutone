package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// HandWritten is what the hand-written Go files of a package declare.
type HandWritten struct {
	// Types are the names of the package-level types.
	Types map[string]bool
	// Methods are the methods, keyed "Receiver.Method" with the receiver's type
	// name and no pointer.
	Methods map[string]bool
}

// ScanHandWritten reads the hand-written Go files in dir. A generator that
// declares one of the things they declare produces a package that does not compile.
//
// Test files and files that carry a "Code generated ... DO NOT EDIT" comment before
// the package clause are not hand-written. A directory that does not exist has no
// hand-written files. One that does not parse is an error, since what it declares
// cannot be known.
func ScanHandWritten(dir string) (HandWritten, error) {
	found := HandWritten{Types: map[string]bool{}, Methods: map[string]bool{}}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return found, nil
	}

	if err != nil {
		return found, err
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		if err := found.scan(filepath.Join(dir, name)); err != nil {
			return found, err
		}
	}

	return found, nil
}

func (h HandWritten) scan(path string) error {
	fset := token.NewFileSet()

	// The header is all that tells a generated file apart, so the body of the
	// large generated files is not parsed.
	head, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		return fmt.Errorf("hand-written file does not parse, so what it declares is unknown: %w", err)
	}

	if isGenerated(head) {
		return nil
	}

	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("hand-written file does not parse, so what it declares is unknown: %w", err)
	}

	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			if v.Recv == nil || len(v.Recv.List) == 0 {
				continue
			}

			if id := receiverIdent(v.Recv.List[0].Type); id != nil {
				h.Methods[id.Name+"."+v.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range v.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					h.Types[ts.Name.Name] = true
				}
			}
		}
	}

	return nil
}

// isGenerated reports whether a comment before the package clause marks the file as
// generated, in the form Go tools recognise but without requiring the final period,
// which Tutone's own header omits.
func isGenerated(f *ast.File) bool {
	for _, group := range f.Comments {
		if group.Pos() >= f.Package {
			break
		}

		for _, c := range group.List {
			text := strings.TrimSuffix(c.Text, ".")

			if strings.HasPrefix(text, "// Code generated ") && strings.HasSuffix(text, " DO NOT EDIT") {
				return true
			}
		}
	}

	return false
}
