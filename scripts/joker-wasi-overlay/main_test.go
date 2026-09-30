package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func initializationOrder(t *testing.T, source []byte) []string {
	t.Helper()
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "fixture.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{}
	if _, err := new(types.Config).Check("fixture", fs, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, init := range info.InitOrder {
		for _, v := range init.Lhs {
			names = append(names, v.Name())
		}
	}
	return names
}

func TestSplitInitializersPreservesDependencies(t *testing.T) {
	source := []byte("package fixture\nvar large [601]int = [601]int{" + strings.Repeat("0, ", 600) + " later}\nvar later int = value()\nvar small []int = []int{later}\nvar scalar int = 7\nvar fn func() int = func() int { return later }\nfunc value() int { return 42 }\n")
	transformed, count, err := splitInitializers(source)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("want one large initializer, got %d", count)
	}
	if !strings.Contains(string(transformed), "//go:noinline\nfunc giWASIBootstrap0") {
		t.Fatal("missing noinline helper")
	}
	if got, want := initializationOrder(t, transformed), initializationOrder(t, source); !reflect.DeepEqual(got, want) {
		t.Fatalf("initialization order changed: %v vs %v", got, want)
	}
	// Every small initializer stays inline to avoid Go's synthetic-init block limit.
	if !strings.Contains(string(transformed), "var small []int = []int{later}") {
		t.Fatal("small initializer extracted")
	}
}

func TestGenerateOverlay(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.go")
	output := filepath.Join(dir, "output.go")
	overlay := filepath.Join(dir, "overlay.json")
	original := []byte("package fixture\nvar large [601]int = [601]int{" + strings.Repeat("0, ", 600) + "0}\n")
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate(source, source, overlay); err == nil {
		t.Fatal("allowed overwrite of original")
	}
	if err := generate(source, output, overlay); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("modified original source")
	}
	if err := os.WriteFile(source, []byte("package fixture\nvar tiny = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate(source, output, overlay); err == nil {
		t.Fatal("should reject changed upstream layout")
	}
}
