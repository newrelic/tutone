package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
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

// TypeStandIn is an identifier that a template printed for a Go type it cannot print as
// one, such as `[]string` or `nrtime.EpochSeconds`, and what to put there instead.
type TypeStandIn struct {
	// Ident is the identifier in the rendered source.
	Ident string
	// Type is the type that replaces Ident where it is used as a type: the type of a
	// field, parameter or result.
	Type string
	// Name replaces Ident everywhere else, such as the name of a field or the field a
	// selector reads.
	Name string
}

// ReplaceTypeStandIns puts the type of each stand-in where the source uses its identifier
// as a type, and its name where it uses it for anything else.
func ReplaceTypeStandIns(src []byte, standIns []TypeStandIn) ([]byte, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "src.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	byIdent := make(map[string]TypeStandIn, len(standIns))
	for _, standIn := range standIns {
		byIdent[standIn.Ident] = standIn
	}

	inType := map[*ast.Ident]bool{}

	ast.Inspect(f, func(n ast.Node) bool {
		if field, ok := n.(*ast.Field); ok {
			ast.Inspect(field.Type, func(t ast.Node) bool {
				if id, ok := t.(*ast.Ident); ok {
					inType[id] = true
				}

				return true
			})
		}

		return true
	})

	var edits []textEdit

	ast.Inspect(f, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		standIn, ok := byIdent[id.Name]
		if !ok {
			return true
		}

		text := standIn.Name
		if inType[id] {
			text = standIn.Type
		}

		edits = append(edits, textEdit{fset.Position(id.Pos()).Offset, fset.Position(id.End()).Offset, text})

		return true
	})

	if len(edits) == 0 {
		return src, nil
	}

	return applyEdits(src, edits), nil
}

// UnsetArguments lists the parameters of a generated function that are left out of the
// request when the caller passes nil.
type UnsetArguments struct {
	// Funcs are the names of the functions that build the request: the method and, for
	// a template that has one, its context.Context variant. A name the source does
	// not declare is skipped.
	Funcs []string
	// Params are the parameters, in the order the checks are written.
	Params []UnsetParam
}

// UnsetParam is a parameter that may be nil and the variable it fills.
type UnsetParam struct {
	// Param is the name of the parameter in the source.
	Param string
	// Key is the key of the variable in the request's variables.
	Key string
}

// OmitUnsetArguments makes a generated method leave a variable out of the request
// when its parameter is nil. Sending the nil as null would override the default the API
// has for the argument.
//
// For each function that declares `vars := map[string]interface{}{...}`, a check is
// written right after that statement for every listed parameter the literal fills:
//
//	if param == nil {
//		delete(vars, "key")
//	}
//
// The names of the functions in specs that none of their Funcs found a `vars` map in
// are returned as unmatched, so the caller can report that the arguments are still sent.
func OmitUnsetArguments(src []byte, specs []UnsetArguments) ([]byte, []string, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "src.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}

	funcs := map[string]*ast.FuncDecl{}

	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			funcs[funcKey(fn)] = fn
		}
	}

	var (
		edits     []textEdit
		unmatched []string
	)

	for _, spec := range specs {
		matched := false

		for _, name := range spec.Funcs {
			fn, ok := funcs["func:"+name]
			if !ok {
				fn, ok = methodNamed(funcs, name)
			}

			if !ok {
				continue
			}

			stmt, lit := varsMap(fn)
			if stmt == nil {
				continue
			}

			matched = true

			var checks strings.Builder

			for _, p := range spec.Params {
				if !fills(lit, p) {
					continue
				}

				fmt.Fprintf(&checks, "\n\tif %s == nil {\n\t\tdelete(vars, %q)\n\t}", p.Param, p.Key)
			}

			if checks.Len() > 0 {
				// A blank line first, so the checks stand apart from the literal.
				at := fset.Position(stmt.End()).Offset
				edits = append(edits, textEdit{at, at, "\n" + checks.String()})
			}
		}

		if !matched && len(spec.Funcs) > 0 {
			unmatched = append(unmatched, spec.Funcs[0])
		}
	}

	if len(edits) == 0 {
		return src, unmatched, nil
	}

	return applyEdits(src, edits), unmatched, nil
}

// methodNamed finds a method by its name alone, whatever its receiver.
func methodNamed(funcs map[string]*ast.FuncDecl, name string) (*ast.FuncDecl, bool) {
	var found *ast.FuncDecl

	for _, fn := range funcs {
		if fn.Recv != nil && fn.Name.Name == name {
			if found != nil {
				return nil, false
			}

			found = fn
		}
	}

	return found, found != nil
}

// varsMap finds the statement `vars := map[...]...{...}` among the top-level statements
// of fn and its literal.
func varsMap(fn *ast.FuncDecl) (*ast.AssignStmt, *ast.CompositeLit) {
	for _, s := range fn.Body.List {
		assign, ok := s.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}

		id, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || id.Name != "vars" {
			continue
		}

		lit, ok := assign.Rhs[0].(*ast.CompositeLit)
		if !ok {
			continue
		}

		if _, isMap := lit.Type.(*ast.MapType); isMap {
			return assign, lit
		}
	}

	return nil, nil
}

// fills reports whether the literal sets the key to the parameter.
func fills(lit *ast.CompositeLit, p UnsetParam) bool {
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.BasicLit)
		if !ok || key.Kind != token.STRING || key.Value != strconv.Quote(p.Key) {
			continue
		}

		if id, ok := kv.Value.(*ast.Ident); ok && id.Name == p.Param {
			return true
		}
	}

	return false
}

// DecodeUnlistedImplementations makes the decoder of each named interface return what it
// cannot match to a listed implementation as the interface's own struct, instead of an
// error. It is for an interface a package narrows to some implementations: the API still
// sends the others, as the fields of the interface give them, and an error here would
// fail a call that has already taken effect.
//
// names are the Go names of the interfaces. For each, the source must declare
// `type <Name> struct` and `func (x *<Name>) Implements<Name>()`, and end
// `Unmarshal<Name>Interface` with the template's "was not matched against all
// PossibleTypes" error; a decoder that does not is left as it is. The branch for a
// response with no __typename is not touched.
func DecodeUnlistedImplementations(src []byte, names []string) ([]byte, error) {
	if len(names) == 0 {
		return src, nil
	}

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "src.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	structs := structTypes(f)
	decoders := map[string]*ast.FuncDecl{}
	marked := map[string]bool{}

	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if fn.Recv == nil {
			decoders[fn.Name.Name] = fn
		} else if id := receiverIdent(fn.Recv.List[0].Type); id != nil {
			marked[id.Name+"."+fn.Name.Name] = true
		}
	}

	var edits []textEdit

	for _, name := range names {
		if _, ok := structs[name]; !ok || !marked[name+".Implements"+name] {
			continue
		}

		fn := decoders["Unmarshal"+name+"Interface"]
		if fn == nil || fn.Body == nil || len(fn.Body.List) == 0 {
			continue
		}

		last := fn.Body.List[len(fn.Body.List)-1]
		if !isNotMatchedReturn(last) {
			continue
		}

		fallback := fmt.Sprintf(`// Not an implementation this package lists: decode the fields they all share.
	var interfaceType %[1]s
	err = json.Unmarshal(b, &interfaceType)
	if err != nil {
		return nil, err
	}

	var xxx %[1]sInterface = &interfaceType

	return &xxx, nil`, name)

		edits = append(edits, textEdit{fset.Position(last.Pos()).Offset, fset.Position(last.End()).Offset, fallback})
	}

	if len(edits) == 0 {
		return src, nil
	}

	return applyEdits(src, edits), nil
}

// isNotMatchedReturn reports whether stmt is the `return nil, fmt.Errorf("interface X was
// not matched against all PossibleTypes: %s", typeName)` that the typegen template ends
// an interface's decoder with.
func isNotMatchedReturn(stmt ast.Stmt) bool {
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 2 {
		return false
	}

	if id, ok := ret.Results[0].(*ast.Ident); !ok || id.Name != "nil" {
		return false
	}

	call, ok := ret.Results[1].(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Errorf" {
		return false
	}

	msg, ok := call.Args[0].(*ast.BasicLit)

	return ok && msg.Kind == token.STRING && strings.Contains(msg.Value, "was not matched against all PossibleTypes")
}
