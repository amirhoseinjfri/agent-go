package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateInventoryFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(aiPath, 0755); err != nil {
		t.Fatal(err)
	}

	src := `// Package demo does demo things.
package demo

// Greeter greets people.
type Greeter struct{}

// Hello says hello.
func (g *Greeter) Hello(name string) string { return "hi " + name }

func helper() {}

const Version = "1"

var Count int
`
	path := filepath.Join(dir, "demo.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateInventoryFromFile(path); err != nil {
		t.Fatal(err)
	}

	pkg, ok := State.Inventory.Packages["demo"]
	if !ok {
		t.Fatalf("package 'demo' not in inventory: %+v", State.Inventory)
	}

	want := map[string]string{
		"Greeter":       "Struct",
		"Greeter.Hello": "Method",
		"helper":        "Function",
		"Version":       "Constant",
		"Count":         "Variable",
	}
	for name, typ := range want {
		item, ok := pkg.Items[name]
		if !ok {
			t.Errorf("missing symbol %q", name)
			continue
		}
		if item.Type != typ {
			t.Errorf("%s: type = %q, want %q", name, item.Type, typ)
		}
	}

	if sig := pkg.Items["Greeter.Hello"].Signature; sig != "func (g *Greeter) Hello(name string) string" {
		t.Errorf("bad method signature: %q", sig)
	}
	if desc := pkg.Items["Greeter"].Description; desc != "Greeter greets people." {
		t.Errorf("bad description: %q", desc)
	}

	// Rewrite the file with a symbol removed; stale entry must disappear.
	src2 := "package demo\n\nfunc helper() {}\n"
	if err := os.WriteFile(path, []byte(src2), 0644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateInventoryFromFile(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := State.Inventory.Packages["demo"].Items["Greeter"]; ok {
		t.Error("stale symbol 'Greeter' was not removed")
	}
}
