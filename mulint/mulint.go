package mulint

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"

	"golang.org/x/tools/go/loader"
)

func Load() *loader.Program {
	var conf loader.Config

	// Use the command-line arguments to specify
	// a set of initial packages to load from source.
	// See FromArgsUsage for help.
	conf.FromArgs(os.Args[1:], false)

	// Finally, load all the packages specified by the configuration.
	prog, _ := conf.Load()

	return prog
}

type PrintVisitor struct{}

func (pv *PrintVisitor) Visit(node ast.Node) ast.Visitor {
	fmt.Printf("Node %#v\n", node)

	return pv
}

type Sequence struct {
	mutexSelector string
	pos           token.Pos
	seq           []ast.Node
}

func NewSequence(mutexSelector string, pos token.Pos) *Sequence {
	return &Sequence{
		mutexSelector: mutexSelector,
		seq:           make([]ast.Node, 0),
		pos:           pos,
	}
}

func (s *Sequence) Pos() token.Pos {
	return s.pos
}

func (s *Sequence) Add(node ast.Node) {
	s.seq = append(s.seq, node)
}

func (s *Sequence) Nodes() []ast.Node {
	return s.seq
}

func (s *Sequence) Selector() string {
	return s.mutexSelector
}

type Sequences struct {
	onGoing  map[string]*Sequence
	defers   map[string]bool
	finished []*Sequence
}

func NewSequences() *Sequences {
	return &Sequences{
		onGoing:  make(map[string]*Sequence),
		defers:   make(map[string]bool),
		finished: make([]*Sequence, 0),
	}
}

func StrExpr(e ast.Expr) string {
	return fmt.Sprintf("%s", e)
}

func (s *Sequences) Track(stmt ast.Stmt) {
	for _, og := range s.onGoing {
		og.Add(stmt)
	}

	// Is start of a sequence to check?
	if e := s.isLockCall(stmt); e != nil {
		se := StrExpr(e)
		if _, exists := s.onGoing[se]; !exists {
			s.onGoing[se] = NewSequence(se, stmt.Pos())
		}
	}

	// Is defer of an unlock?
	if e := s.isDeferUnlockCall(stmt); e != nil {
		se := StrExpr(e)
		s.defers[se] = true
	}

	// Is end of a sequence to check?
	if e := s.isUnlockCall(stmt); e != nil {
		se := StrExpr(e)
		if ogs, ok := s.onGoing[se]; ok {
			s.finished = append(s.finished, ogs)
		}
	}
}

func (s *Sequences) EndBlock() {
	for k, _ := range s.defers {
		s.finished = append(s.finished, s.onGoing[k])
	}

	s.onGoing = make(map[string]*Sequence)
	s.defers = make(map[string]bool)
}

func (s *Sequences) Sequences() []*Sequence {
	return s.finished
}

func (s *Sequences) isLockCall(node ast.Node) ast.Expr {
	return SubjectForCall(node, []string{"RLock", "Lock"})
}

func (s *Sequences) isUnlockCall(node ast.Node) ast.Expr {
	return SubjectForCall(node, []string{"RUnlock", "Unlock"})
}

func (s *Sequences) isDeferUnlockCall(node ast.Node) ast.Expr {
	switch sty := node.(type) {
	case *ast.DeferStmt:
		return s.isUnlockCall(sty.Call)
	}

	return nil
}

func SubjectForCall(node ast.Node, names []string) ast.Expr {
	switch sty := node.(type) {
	case *ast.CallExpr:
		selector := SelectorExpr(sty)
		fnName := selector.Sel.Name

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
		fnName := selector.Sel.Name

		for _, name := range names {
			if name == fnName {
				return selector.X
			}
		}
	default:
	}

	return nil
}

func SelectorExpr(call *ast.CallExpr) *ast.SelectorExpr {
	switch exp := call.Fun.(type) {
	case (*ast.SelectorExpr):
		return exp
	default:
	}

	return nil
}

type Visitor struct {
	sequences *Sequences
	program   *loader.Program
}

func NewVisitor(prog *loader.Program) *Visitor {
	return &Visitor{
		sequences: NewSequences(),
		program:   prog,
	}
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	switch stmt := node.(type) {
	case *ast.FuncDecl:
		body := stmt.Body
		v.analyzeBody(body)
	default:
	}
	return v
}

func (v *Visitor) Sequences() []*Sequence {
	return v.sequences.Sequences()
}

func (v *Visitor) analyzeBody(body *ast.BlockStmt) {
	for _, stmt := range body.List {
		v.sequences.Track(stmt)
	}

	v.sequences.EndBlock()
}
