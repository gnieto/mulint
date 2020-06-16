package mulint

import (
	"go/ast"
	"go/token"

	"github.com/securego/gosec"
	"golang.org/x/tools/go/loader"
)

type Analyzer struct {
	errors []LintError
	pkg    *loader.PackageInfo
	scopes map[FQN]*Scopes
	calls  map[FQN][]FQN
}

func NewAnalyzer(pkg *loader.PackageInfo, scopes map[FQN]*Scopes, calls map[FQN][]FQN) *Analyzer {
	return &Analyzer{
		pkg:    pkg,
		scopes: scopes,
		calls:  calls,
	}
}

func (a *Analyzer) Errors() []LintError {
	return a.errors
}

func (a *Analyzer) Analyze() {
	for _, s := range a.scopes {
		for _, seq := range s.Scopes() {
			for _, n := range seq.Nodes() {
				a.ContainsLock(n, seq)
			}
		}
	}
}

func (a *Analyzer) ContainsLock(n ast.Node, seq *MutexScope) {
	switch sty := n.(type) {
	case *ast.ExprStmt:
		a.ContainsLock(sty.X, seq)
	case *ast.CallExpr:
		a.checkLockToSequenceMutex(seq, sty)
		a.checkCallToFuncWhichLocksSameMutex(seq, sty)
	}
}

func (a *Analyzer) checkCallToFuncWhichLocksSameMutex(seq *MutexScope, callExpr *ast.CallExpr) {
	ctx := &gosec.Context{
		Pkg:  a.pkg.Pkg,
		Info: &a.pkg.Info,
	}

	pkg, name, err := gosec.GetCallInfo(callExpr, ctx)

	if err == nil {
		fqn := FromCallInfo(pkg, name)

		if a.hasTransitiveCall(fqn, seq, make(map[FQN]bool)) == true {
			a.recordError(seq.Pos(), callExpr.Pos())
		}
	}
}

func (a *Analyzer) hasAnyMutexScopeWithSameSelector(fqn FQN, seq *MutexScope) bool {
	mutexScopes, ok := a.scopes[fqn]
	if !ok {
		return false
	}

	for _, currentMutexScope := range mutexScopes.Scopes() {
		if currentMutexScope.IsEqual(seq) == true {
			return true
		}
	}

	return false
}

func (a *Analyzer) hasTransitiveCall(fqn FQN, seq *MutexScope, checked map[FQN]bool) bool {
	if checked, ok := checked[fqn]; ok {
		return checked
	}

	if hasLock := a.hasAnyMutexScopeWithSameSelector(fqn, seq); hasLock {
		checked[fqn] = hasLock

		return hasLock
	}

	calls, ok := a.calls[fqn]
	if !ok {
		return false
	}

	any := false
	for _, c := range calls {
		any = any || a.hasTransitiveCall(c, seq, checked)
	}

	return any
}

func (a *Analyzer) checkLockToSequenceMutex(seq *MutexScope, callExpr *ast.CallExpr) {
	selector := StrExpr(SubjectForCall(callExpr, []string{"RLock", "Lock"}))

	if selector == seq.Selector() {
		a.recordError(seq.Pos(), callExpr.Pos())
	}
}

func (a *Analyzer) recordError(origin token.Pos, secondLock token.Pos) {
	originLoc := NewLocation(origin)
	secondLockLoc := NewLocation(secondLock)

	err := NewLintError(originLoc, secondLockLoc)
	a.errors = append(a.errors, err)
}
