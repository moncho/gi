// Command joker-wasi-overlay splits Joker's generated bootstrap initializers
// for WASI builds, without editing the upstream module or module cache.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

func splitInitializers(source []byte) ([]byte, int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "bootstrap.go", source, parser.ParseComments)
	if err != nil {
		return nil, 0, err
	}
	var helpers []ast.Decl
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, spec := range g.Specs {
			v := spec.(*ast.ValueSpec)
			if len(v.Names) != 1 || len(v.Values) != 1 || v.Type == nil {
				continue
			}
			// Scalars need no helper. Function literals remain untouched: their bodies
			// are already separate functions, not large bootstrap assignments.
			switch v.Values[0].(type) {
			case *ast.BasicLit, *ast.Ident, *ast.FuncLit:
				continue
			}
			// Leave small expressions inline: extracting tens of thousands of tiny
			// values makes Go's synthetic init exceed its basic-block limit.
			if fset.Position(v.Values[0].End()).Offset-fset.Position(v.Values[0].Pos()).Offset < 512 {
				continue
			}
			name := fmt.Sprintf("giWASIBootstrap%d", len(helpers))
			helpers = append(helpers, &ast.FuncDecl{
				Doc:  &ast.CommentGroup{List: []*ast.Comment{{Text: "//go:noinline"}}},
				Name: ast.NewIdent(name),
				Type: &ast.FuncType{Params: &ast.FieldList{}, Results: &ast.FieldList{List: []*ast.Field{{Type: v.Type}}}},
				Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: v.Values}}},
			})
			v.Values = []ast.Expr{&ast.CallExpr{Fun: ast.NewIdent(name)}}
		}
	}
	// Go's initialization dependency analysis follows referenced function bodies,
	// so these helpers retain dependencies, declaration order and evaluation count.
	// Format helpers separately: synthetic comment positions must not attach the
	// noinline directive to an existing declaration with an original source position.
	var out bytes.Buffer
	if err := format.Node(&out, fset, f); err != nil {
		return nil, 0, err
	}
	for _, h := range helpers {
		fn := h.(*ast.FuncDecl)
		fn.Doc = nil
		out.WriteString("\n//go:noinline\n")
		if err := format.Node(&out, token.NewFileSet(), fn); err != nil {
			return nil, 0, err
		}
		out.WriteByte('\n')
	}
	formatted, err := format.Source(out.Bytes())
	return formatted, len(helpers), err
}

func main() {
	source := flag.String("source", "", "upstream generated bootstrap file")
	output := flag.String("output", "", "transformed file")
	overlay := flag.String("overlay", "", "Go build overlay JSON")
	flag.Parse()
	if *source == "" || *output == "" || *overlay == "" {
		fmt.Fprintln(os.Stderr, "source, output and overlay are required")
		os.Exit(1)
	}
	if err := generate(*source, *output, *overlay); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(source, output, overlay string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if source == output {
		return fmt.Errorf("refusing to overwrite upstream source")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	data, count, err := splitInitializers(data)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no initializers found; upstream layout may have changed")
	}
	if err := os.WriteFile(output, data, 0600); err != nil {
		return err
	}
	mapping, err := json.Marshal(struct{ Replace map[string]string }{map[string]string{source: output}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(overlay, mapping, 0600); err != nil {
		return err
	}
	fmt.Printf("Split %d Joker bootstrap initializers for WASI\n", count)
	return nil
}
