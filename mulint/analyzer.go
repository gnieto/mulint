package mulint

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/GoASTScanner/gas"
	"golang.org/x/tools/go/loader"
)

type Analyzer struct {
	errors    []LintError
	pkg       *loader.PackageInfo
	sequences map[FQN]*Sequences
	calls     map[FQN][]FQN
}

func NewAnalyzer(pkg *loader.PackageInfo, sequences map[FQN]*Sequences, calls map[FQN][]FQN) *Analyzer {
	return &Analyzer{
		pkg:       pkg,
		sequences: sequences,
		calls:     calls,
	}
}

func (a *Analyzer) Errors() []LintError {
	return a.errors
}

func (a *Analyzer) Analyze() {
	for _, s := range a.sequences {
		for _, seq := range s.Sequences() {
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
	ctx := &gas.Context{
		Pkg:  a.pkg.Pkg,
		Info: &a.pkg.Info,
	}

	pkg, name, err := gas.GetCallInfo(callExpr, ctx)

	if err == nil {
		fqn := FQN(strings.Trim(fmt.Sprintf("%s:%s", pkg, name), "*"))

		if a.hasTransitiveCall(fqn, seq, make(map[FQN]bool)) == true {
			a.recordError(seq.Pos(), callExpr.Pos())
		}
	}
}

func (a *Analyzer) hasAnyMutexScopeWithSameSelector(fqn FQN, seq *MutexScope) bool {
	mutexScopes, ok := a.sequences[fqn]
	if !ok {
		return false
	}

	for _, currentMutexScope := range mutexScopes.Sequences() {
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
