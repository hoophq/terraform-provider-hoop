// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chokepointFiles are the only files in this package allowed to build HTTP
// requests, set HTTP headers or reach the underlying HTTP client.
var chokepointFiles = map[string]bool{
	"client.go": true,
	"auth.go":   true,
}

// TestRequestChokepoint keeps every gateway call flowing through
// (*Client).newRequest and (*Client).send.
//
// The 401 fixed by EVL-156 happened because each of the 24 endpoints built its
// own request and hardcoded the legacy Api-Key header, which the gateway
// short-circuits on before it ever looks at the bearer token. Centralising
// request construction makes that class of bug unrepresentable: this test fails
// as soon as a new endpoint sets a header of its own.
func TestRequestChokepoint(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed reading package directory: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if chokepointFiles[name] {
			continue
		}
		checked++
		assertNoRequestPlumbing(t, name)
	}

	if checked == 0 {
		t.Fatal("no files were inspected, the chokepoint guard is not testing anything")
	}
}

func assertNoRequestPlumbing(t *testing.T, filename string) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Clean(filename), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("failed parsing %s: %v", filename, err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pos := fset.Position(call.Pos())

		// http.NewRequest / http.NewRequestWithContext
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "http" && strings.HasPrefix(sel.Sel.Name, "NewRequest") {
			t.Errorf("%s: builds its own request with http.%s; use (*Client).newRequest so the credential header is set in one place",
				pos, sel.Sel.Name)
			return true
		}

		inner, ok := sel.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// <something>.Header.Set / Add / Del
		switch sel.Sel.Name {
		case "Set", "Add", "Del":
			if inner.Sel.Name == "Header" {
				t.Errorf("%s: sets an HTTP header directly; headers belong in (*Client).newRequest", pos)
				return true
			}
		}

		// c.httpClient.Do
		if sel.Sel.Name == "Do" && inner.Sel.Name == "httpClient" {
			t.Errorf("%s: calls the HTTP client directly; use (*Client).send or (*Client).sendDiscard", pos)
		}
		return true
	})
}
