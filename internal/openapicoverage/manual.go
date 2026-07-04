package openapicoverage

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var httpMethods = map[string]string{
	"MethodGet":     "GET",
	"MethodPost":    "POST",
	"MethodPut":     "PUT",
	"MethodDelete":  "DELETE",
	"MethodPatch":   "PATCH",
	"MethodHead":    "HEAD",
	"MethodOptions": "OPTIONS",
}

// ParseManualRouteDir parses hand-written HTTP handlers and returns exact
// /api routes that complement generated grpc-gateway routes.
func ParseManualRouteDir(dir string) ([]GatewayRoute, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", dir, err)
	}

	seen := make(map[string]GatewayRoute)
	for _, file := range matches {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		routes, err := parseManualRouteFile(file)
		if err != nil {
			return nil, err
		}
		for _, r := range routes {
			seen[routeIdentity(r)] = r
		}
	}

	out := make([]GatewayRoute, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].HTTPMethod < out[j].HTTPMethod
	})
	return out, nil
}

func parseManualRouteFile(path string) ([]GatewayRoute, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse manual routes %q: %w", path, err)
	}

	var routes []GatewayRoute
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt:
			routes = append(routes, routesFromIf(n, "")...)
		case *ast.SwitchStmt:
			routes = append(routes, routesFromSwitch(n, "")...)
			return false
		}
		return true
	})
	return routes, nil
}

func routesFromStmtList(stmts []ast.Stmt, method string) []GatewayRoute {
	var routes []GatewayRoute
	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case *ast.BlockStmt:
			routes = append(routes, routesFromStmtList(stmt.List, method)...)
		case *ast.IfStmt:
			routes = append(routes, routesFromIf(stmt, method)...)
		case *ast.SwitchStmt:
			routes = append(routes, routesFromSwitch(stmt, method)...)
		}
	}
	return routes
}

func routesFromIf(stmt *ast.IfStmt, inheritedMethod string) []GatewayRoute {
	method, path := routeConditionParts(stmt.Cond)
	if method == "" {
		method = inheritedMethod
	}

	var routes []GatewayRoute
	if method != "" && isAPIRoute(path) {
		routes = append(routes, manualRoute(method, path))
	}
	routes = append(routes, routesFromStmtList(stmt.Body.List, method)...)
	if stmt.Else != nil {
		routes = append(routes, routesFromElse(stmt.Else, inheritedMethod)...)
	}
	return routes
}

func routesFromElse(stmt ast.Stmt, method string) []GatewayRoute {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		return routesFromStmtList(stmt.List, method)
	case *ast.IfStmt:
		return routesFromIf(stmt, method)
	default:
		return nil
	}
}

func routesFromSwitch(stmt *ast.SwitchStmt, inheritedMethod string) []GatewayRoute {
	var routes []GatewayRoute
	switch {
	case isMethodSelector(stmt.Tag):
		for _, s := range stmt.Body.List {
			clause := s.(*ast.CaseClause)
			for _, expr := range clause.List {
				method := httpMethod(expr)
				if method == "" {
					continue
				}
				routes = append(routes, routesFromStmtList(clause.Body, method)...)
			}
		}
	case isPathSelector(stmt.Tag):
		for _, s := range stmt.Body.List {
			clause := s.(*ast.CaseClause)
			for _, expr := range clause.List {
				path := stringLiteral(expr)
				if inheritedMethod != "" && isAPIRoute(path) {
					routes = append(routes, manualRoute(inheritedMethod, path))
				}
			}
			routes = append(routes, routesFromStmtList(clause.Body, inheritedMethod)...)
		}
	default:
		for _, s := range stmt.Body.List {
			clause := s.(*ast.CaseClause)
			routes = append(routes, routesFromStmtList(clause.Body, inheritedMethod)...)
		}
	}
	return routes
}

func routeConditionParts(expr ast.Expr) (method, path string) {
	switch expr := expr.(type) {
	case *ast.BinaryExpr:
		if expr.Op == token.LAND {
			leftMethod, leftPath := routeConditionParts(expr.X)
			rightMethod, rightPath := routeConditionParts(expr.Y)
			if leftMethod != "" {
				method = leftMethod
			} else {
				method = rightMethod
			}
			if leftPath != "" {
				path = leftPath
			} else {
				path = rightPath
			}
			return method, path
		}
		if expr.Op != token.EQL {
			return "", ""
		}
		return routeEqualityParts(expr.X, expr.Y)
	default:
		return "", ""
	}
}

func routeEqualityParts(left, right ast.Expr) (method, path string) {
	if isMethodSelector(left) {
		return httpMethod(right), ""
	}
	if isMethodSelector(right) {
		return httpMethod(left), ""
	}
	if isPathSelector(left) {
		return "", stringLiteral(right)
	}
	if isPathSelector(right) {
		return "", stringLiteral(left)
	}
	return "", ""
}

func isMethodSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Method"
}

func isPathSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Path" {
		return false
	}
	parent, ok := sel.X.(*ast.SelectorExpr)
	return ok && parent.Sel.Name == "URL"
}

func httpMethod(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return httpMethods[sel.Sel.Name]
}

func stringLiteral(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return value
}

func isAPIRoute(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func manualRoute(method, path string) GatewayRoute {
	return GatewayRoute{
		Service:    "ManualHTTP",
		Method:     manualRouteName(method, path),
		HTTPMethod: method,
		Path:       path,
	}
}

func manualRouteName(method, path string) string {
	var b strings.Builder
	b.WriteString("Handle")
	b.WriteString(method)
	for _, part := range strings.FieldsFunc(path, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if part == "" {
			continue
		}
		for i, r := range part {
			if i == 0 {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
