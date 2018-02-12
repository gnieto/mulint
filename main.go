package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"reflect"

	"github.com/gnieto/mulint/mulint"
	"golang.org/x/tools/go/loader"
)

func main() {
	fmt.Println("aaa")
	// v := &mulint.PrintVisitor{}
	p := mulint.Load()
	pkg := p.Package("github.com/gnieto/mulint/tests")
	v := mulint.NewVisitor(p, pkg)

	for _, file := range pkg.Files {
		ast.Walk(v, file)
		seqs := v.Sequences()
		analyzer := NewAnalyzer()

		for _, s := range seqs {
			analyzer.Analyze(s)
		}

		report(p, analyzer.Errors())
	}
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

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
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
	errors []LintError
}

func (a *Analyzer) Errors() []LintError {
	return a.errors
}

func (a *Analyzer) Analyze(seq *mulint.MutexScope) {
	fmt.Println("Start analyzing sequence!!")
	for _, n := range seq.Nodes() {
		fmt.Println("Stamentent", reflect.TypeOf(n))
		a.ContainsLock(n, seq)
	}

	fmt.Println()
	fmt.Println()
	fmt.Println()
}

func (a *Analyzer) ContainsLock(n ast.Node, seq *mulint.MutexScope) {
	switch sty := n.(type) {
	case *ast.ExprStmt:
		a.ContainsLock(sty.X, seq)
	case *ast.CallExpr:
		a.checkLockToSequenceMutex(seq, sty)
	}
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
