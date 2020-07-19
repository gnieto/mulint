package mulint

import (
	"go/ast"
)

func CallExpr(node ast.Node) *ast.CallExpr {
	switch sty := node.(type) {
	case *ast.CallExpr:
		return sty
	case *ast.ExprStmt:
		exp, ok := sty.X.(*ast.CallExpr)

		if ok {
			return exp
		}
	}

	return nil
}

func SubjectForCall(node ast.Node, names []string) ast.Expr {
	switch sty := node.(type) {
	case *ast.CallExpr:
		selector := SelectorExpr(sty)

		fnName := ""
		if selector != nil {
			fnName = selector.Sel.Name
		}

		for _, name := range names {
			if name == fnName {
				return selector.X
			}
		}
	case *ast.ExprStmt:
		exp, ok := sty.X.(*ast.CallExpr)
		if !ok {
			return nil
		}

		selector := SelectorExpr(exp)
		fnName := ""
		if selector != nil {
			fnName = selector.Sel.Name
		}

		for _, name := range names {
			if name == fnName {
				return selector.X
			}
		}
	default:
	}

	return nil
}

func RootSelector(sel *ast.SelectorExpr) *ast.Ident {
	switch sty := sel.X.(type) {
	case *ast.SelectorExpr:
		return RootSelector(sty)
	case *ast.Ident:
		return sty
	}

	return nil
}

func SelectorExpr(call *ast.CallExpr) *ast.SelectorExpr {
	switch exp := call.Fun.(type) {
	case *ast.SelectorExpr:
		return exp
	default:
	}

	return nil
}
