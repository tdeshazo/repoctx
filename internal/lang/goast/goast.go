package goast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

type Result struct {
	Unit    string
	Nodes   []ir.Node
	Roots   []int
	Symbols []LocalSymbol
	Edges   []LocalEdge
}

type LocalSymbol struct {
	Name     string
	Kind     ir.SymbolKind
	Node     int
	Parent   int // local symbol index + 1; zero means none
	Receiver string
}

type LocalEdge struct {
	Kind ir.EdgeKind
	Node int
	Text string
}

func Parse(src []byte, st *ir.Strings) (Result, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "input.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return Result{}, err
	}

	r := Result{Unit: f.Name.Name}
	nodeIndex := map[ast.Node]int{}
	var add func(ast.Node) int
	add = func(n ast.Node) int {
		if n == nil {
			return -1
		}
		idx := len(r.Nodes)
		nodeIndex[n] = idx
		r.Nodes = append(r.Nodes, ir.Node{
			Kind: st.Intern(kindOf(n)),
			Text: st.Intern(textOf(n)),
			Span: span(fset, n),
		})
		for _, child := range directChildren(n) {
			ci := add(child)
			if ci >= 0 {
				r.Nodes[idx].Children = append(r.Nodes[idx].Children, ci)
			}
		}
		return idx
	}
	r.Roots = append(r.Roots, add(f))

	// A compact symbol and edge layer complements the normalized AST.
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					r.Symbols = append(r.Symbols, LocalSymbol{Name: s.Name.Name, Kind: ir.SymType, Node: nodeIndex[s]})
				case *ast.ValueSpec:
					k := ir.SymVariable
					if x.Tok == token.CONST {
						k = ir.SymConstant
					}
					for _, name := range s.Names {
						r.Symbols = append(r.Symbols, LocalSymbol{Name: name.Name, Kind: k, Node: nodeIndex[s]})
					}
				case *ast.ImportSpec:
					path, _ := strconv.Unquote(s.Path.Value)
					r.Edges = append(r.Edges, LocalEdge{Kind: ir.EdgeImports, Node: nodeIndex[s], Text: path})
				}
			}
		case *ast.FuncDecl:
			k := ir.SymFunction
			if x.Recv != nil {
				k = ir.SymMethod
			}
			r.Symbols = append(r.Symbols, LocalSymbol{Name: x.Name.Name, Kind: k, Node: nodeIndex[x], Receiver: receiverName(x)})
		case *ast.CallExpr:
			if name := callName(x.Fun); name != "" {
				r.Edges = append(r.Edges, LocalEdge{Kind: ir.EdgeCalls, Node: nodeIndex[x], Text: name})
			}
		}
		return true
	})
	types := map[string]int{}
	for i, s := range r.Symbols {
		if s.Kind == ir.SymType {
			types[s.Name] = i + 1
		}
	}
	for i := range r.Symbols {
		if p := r.Symbols[i].Receiver; p != "" {
			r.Symbols[i].Parent = types[p]
		}
	}
	return r, nil
}

func receiverName(f *ast.FuncDecl) string {
	if f.Recv == nil || len(f.Recv.List) == 0 {
		return ""
	}
	x := f.Recv.List[0].Type
	if star, ok := x.(*ast.StarExpr); ok {
		x = star.X
	}
	switch t := x.(type) {
	case *ast.IndexExpr:
		x = t.X
	case *ast.IndexListExpr:
		x = t.X
	}
	if id, ok := x.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func kindOf(n ast.Node) string {
	t := reflect.TypeOf(n)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func textOf(n ast.Node) string {
	switch x := n.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.BasicLit:
		return x.Value
	case *ast.ImportSpec:
		return x.Path.Value
	case *ast.FuncDecl:
		return x.Name.Name
	case *ast.TypeSpec:
		return x.Name.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.Field:
		if len(x.Names) > 0 {
			return x.Names[0].Name
		}
	}
	return ""
}

func span(fset *token.FileSet, n ast.Node) ir.Span {
	a := fset.PositionFor(n.Pos(), false)
	b := fset.PositionFor(n.End(), false)
	return ir.Span{SL: a.Line, SC: max(0, a.Column-1), EL: b.Line, EC: max(0, b.Column-1)}
}

func directChildren(n ast.Node) []ast.Node {
	var out []ast.Node
	ast.Inspect(n, func(c ast.Node) bool {
		if c == nil || c == n {
			return true
		}
		out = append(out, c)
		return false
	})
	return out
}

func callName(n ast.Expr) string {
	switch x := n.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if p, ok := x.X.(*ast.Ident); ok {
			return p.Name + "." + x.Sel.Name
		}
		return x.Sel.Name
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
