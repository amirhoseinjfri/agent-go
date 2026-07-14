package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"maps"
	"strings"
)

// UpdateInventoryFromFile parses a Go source file and refreshes its symbols in
// INVENTORY.yaml. Non-Go files are ignored. Called automatically after every
// write_file/patch_file, so the inventory never drifts from the code.
func UpdateInventoryFromFile(path string) error {
	if !strings.HasSuffix(path, ".go") {
		return nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if err := LoadInventory(); err != nil {
		return err
	}
	if State.Inventory.Packages == nil {
		State.Inventory.Packages = map[string]PackageData{}
	}

	phase := GetCurrentPhaseID()
	items := map[string]CodeItem{}
	add := func(name, typ, sig string, doc *ast.CommentGroup) {
		items[name] = CodeItem{
			Type:           typ,
			Signature:      sig,
			Description:    docLine(doc),
			FilePath:       path,
			CreatedInPhase: phase,
		}
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name, typ := d.Name.Name, "Function"
			if d.Recv != nil {
				typ = "Method"
				name = receiverName(d.Recv) + "." + name
			}
			add(name, typ, funcSignature(fset, d), d.Doc)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					typ, kind := "Struct", "struct"
					switch s.Type.(type) {
					case *ast.StructType:
					case *ast.InterfaceType:
						typ, kind = "Interface", "interface"
					default:
						typ, kind = "Type", ""
					}
					add(s.Name.Name, typ, strings.TrimSpace("type "+s.Name.Name+" "+kind), pickDoc(s.Doc, d.Doc))
				case *ast.ValueSpec:
					typ, kw := "Variable", "var "
					if d.Tok == token.CONST {
						typ, kw = "Constant", "const "
					}
					for _, n := range s.Names {
						if n.Name == "_" {
							continue
						}
						add(n.Name, typ, kw+n.Name, pickDoc(s.Doc, d.Doc))
					}
				}
			}
		}
	}

	// Drop stale entries for this file, then re-add the fresh ones.
	for pkgName, pkg := range State.Inventory.Packages {
		for k, it := range pkg.Items {
			if it.FilePath == path {
				delete(pkg.Items, k)
			}
		}
		if len(pkg.Items) == 0 {
			delete(State.Inventory.Packages, pkgName)
		}
	}

	if len(items) > 0 {
		pkg, ok := State.Inventory.Packages[f.Name.Name]
		if !ok || pkg.Items == nil {
			desc := docLine(f.Doc)
			if desc == "" {
				desc = "Package " + f.Name.Name
			}
			pkg = PackageData{Description: desc, Items: map[string]CodeItem{}}
		}
		maps.Copy(pkg.Items, items)
		State.Inventory.Packages[f.Name.Name] = pkg
	}

	return SaveInventory()
}

// funcSignature renders a declaration without its body, collapsed to one line.
func funcSignature(fset *token.FileSet, d *ast.FuncDecl) string {
	cp := *d
	cp.Body = nil
	cp.Doc = nil
	var b strings.Builder
	_ = printer.Fprint(&b, fset, &cp)
	return strings.Join(strings.Fields(b.String()), " ")
}

func receiverName(recv *ast.FieldList) string {
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	switch tt := t.(type) {
	case *ast.Ident:
		return tt.Name
	case *ast.IndexExpr: // generic receiver like Box[T]
		if id, ok := tt.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return "unknown"
}

func pickDoc(specific, general *ast.CommentGroup) *ast.CommentGroup {
	if specific != nil {
		return specific
	}
	return general
}

func docLine(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(doc.Text()), "\n")
	return line
}
