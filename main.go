package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"reflect"
	"strings"

	"github.com/GoASTScanner/gas"
	"github.com/gnieto/mulint/mulint"
	"golang.org/x/tools/go/loader"
)

func main() {
	p := mulint.Load()
	pkg := p.Package("github.com/gnieto/mulint/tests")
	v := mulint.NewVisitor(p, pkg)

	for _, file := range pkg.Files {
		ast.Walk(v, file)
	}

	a := NewAnalyzer(pkg, v.Sequences(), v.Calls())
	a.Analyze()

	report(p, a.Errors())
}

func report(p *loader.Program, errors []LintError) {
	for _, e := range errors {
		secondLockPosition := p.Fset.Position(e.secondLock.pos)
		secondLockLine := getLine(p, secondLockPosition)
		originLockPosition := p.Fset.Position(e.origin.pos)
		originLine := getLine(p, originLockPosition)

		fmt.Printf("%s:[%d] Mutex is adquired on this line: %s\n", secondLockPosition.Filename, secondLockPosition.Line, secondLockLine)
		fmt.Printf("\t%s:[%d] But the same mutex already acquired a lock on the following line, and this will cause a dead-lock: %s\n", originLockPosition.Filename, originLockPosition.Line, originLine)
	}
}

func getLine(p *loader.Program, position token.Position) string {
	lines := readfile(position.Filename)

	return lines[position.Line-1]
}

// From errcheck
func readfile(filename string) []string {
	var f, err = os.Open(filename)
	if err != nil {
		return nil
	}

	var lines []string
	var scanner = bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func NewAnalyzer(pkg *loader.PackageInfo, sequences map[mulint.FQN]*mulint.Sequences, calls map[mulint.FQN][]mulint.FQN) *Analyzer {
	return &Analyzer{
		pkg:       pkg,
		sequences: sequences,
		calls:     calls,
	}
}

type Location struct {
	pos token.Pos
}

func NewLocation(pos token.Pos) Location {
	return Location{
		pos: pos,
	}
}

type LintError struct {
	origin     Location
	secondLock Location
}

func NewLintError(origin Location, secondLock Location) LintError {
	return LintError{
		origin:     origin,
		secondLock: secondLock,
	}
}

type Analyzer struct {
	errors    []LintError
	pkg       *loader.PackageInfo
	sequences map[mulint.FQN]*mulint.Sequences
	calls     map[mulint.FQN][]mulint.FQN
}

func (a *Analyzer) Errors() []LintError {
	return a.errors
}

func (a *Analyzer) Analyze() {
	for _, s := range a.sequences {
		for _, seq := range s.Sequences() {
			fmt.Println("Start analyzing sequence!!")
			for _, n := range seq.Nodes() {
				fmt.Println("Stamentent", reflect.TypeOf(n))
				a.ContainsLock(n, seq)
			}

			fmt.Println()
			fmt.Println()
			fmt.Println()
		}
	}
}

func (a *Analyzer) ContainsLock(n ast.Node, seq *mulint.MutexScope) {
	switch sty := n.(type) {
	case *ast.ExprStmt:
		a.ContainsLock(sty.X, seq)
	case *ast.CallExpr:
		a.checkLockToSequenceMutex(seq, sty)
		a.checkCallToFuncWhichLocksSameMutex(seq, sty)
	default:
		fmt.Println("No ContainLocks for ", reflect.TypeOf(n))
	}
}

func (a *Analyzer) checkCallToFuncWhichLocksSameMutex(seq *mulint.MutexScope, callExpr *ast.CallExpr) {
	ctx := &gas.Context{
		Pkg:  a.pkg.Pkg,
		Info: &a.pkg.Info,
	}

	pkg, name, err := gas.GetCallInfo(callExpr, ctx)

	if err == nil {
		fqn := mulint.FQN(strings.Trim(fmt.Sprintf("%s:%s", pkg, name), "*"))
		fmt.Println("On the scope of ", seq.Selector(), " call to ", pkg, name)

		if a.hasTransitiveCall(fqn, seq, make(map[mulint.FQN]bool)) == true {
			a.recordError(seq.Pos(), callExpr.Pos())
		}
	}
}

func (a *Analyzer) hasAnyMutexScopeWithSameSelector(fqn mulint.FQN, seq *mulint.MutexScope) bool {
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

func (a *Analyzer) hasTransitiveCall(fqn mulint.FQN, seq *mulint.MutexScope, checked map[mulint.FQN]bool) bool {
	fmt.Println("\tHas transitive call? -> ", fqn)
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

func (a *Analyzer) checkLockToSequenceMutex(seq *mulint.MutexScope, callExpr *ast.CallExpr) {
	selector := mulint.StrExpr(mulint.SubjectForCall(callExpr, []string{"RLock", "Lock"}))

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
