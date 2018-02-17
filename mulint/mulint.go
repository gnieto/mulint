package mulint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"strings"

	"github.com/GoASTScanner/gas"
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

type MutexScope struct {
	mutexSelector string
	pos           token.Pos
	seq           []ast.Node
	v             *types.Var
}

func NewMutexScope(mutexSelector string, pos token.Pos, fn *types.Var) *MutexScope {
	return &MutexScope{
		mutexSelector: mutexSelector,
		seq:           make([]ast.Node, 0),
		pos:           pos,
		v:             fn,
	}
}

func (s *MutexScope) Pos() token.Pos {
	return s.pos
}

func (s *MutexScope) Add(node ast.Node) {
	s.seq = append(s.seq, node)
}

func (s *MutexScope) Nodes() []ast.Node {
	return s.seq
}

func (s *MutexScope) IsEqual(right *MutexScope) bool {
	return s.mutexSelector == right.mutexSelector
}

func (s *MutexScope) Selector() string {
	return s.mutexSelector
}

func (s *MutexScope) IsSameType(v *types.Var) bool {
	return v != nil && s.v != nil && s.v.String() == v.String()
}

type Sequences struct {
	onGoing  map[string]*MutexScope
	defers   map[string]bool
	finished []*MutexScope
	prog     *loader.Program
	pkg      *loader.PackageInfo
}

func NewSequences(prog *loader.Program, pkg *loader.PackageInfo) *Sequences {
	return &Sequences{
		onGoing:  make(map[string]*MutexScope),
		defers:   make(map[string]bool),
		finished: make([]*MutexScope, 0),
		prog:     prog,
		pkg:      pkg,
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
			call := CallExpr(stmt)
			sel := SelectorExpr(call)
			root := RootSelector(sel)
			ty, _ := s.pkg.ObjectOf(root).(*types.Var)

			if true {
				s.onGoing[se] = NewMutexScope(se, stmt.Pos(), ty)
			}

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

	s.onGoing = make(map[string]*MutexScope)
	s.defers = make(map[string]bool)
}

func (s *Sequences) HasAnyScope() bool {
	return len(s.finished) > 0
}

func (s *Sequences) Sequences() []*MutexScope {
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
	case (*ast.SelectorExpr):
		return exp
	default:
	}

	return nil
}

type FQN string

type Visitor struct {
	sequences map[FQN]*Sequences
	calls     map[FQN][]FQN
	program   *loader.Program
	pkg       *loader.PackageInfo
}

func NewVisitor(prog *loader.Program, pkg *loader.PackageInfo) *Visitor {
	return &Visitor{
		sequences: make(map[FQN]*Sequences),
		calls:     make(map[FQN][]FQN),
		program:   prog,
		pkg:       pkg,
	}
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	switch stmt := node.(type) {
	case *ast.FuncDecl:
		body := stmt.Body
		fqn := FQN(v.fqn(stmt))

		v.analyzeBody(fqn, body)
		v.recordCalls(fqn, body)
	default:
	}
	return v
}

func (v *Visitor) recordCalls(currentFQN FQN, body *ast.BlockStmt) {
	for _, stmt := range body.List {
		call := CallExpr(stmt)

		if call != nil {
			ctx := gas.Context{
				Pkg:  v.pkg.Pkg,
				Info: &v.pkg.Info,
			}

			pkg, name, err := gas.GetCallInfo(call, &ctx)
			if err == nil {
				fqn := fmt.Sprintf("%s:%s", strings.Trim(pkg, "*"), name)
				v.addCall(currentFQN, FQN(fqn))
			}
		}
	}
}

func (v *Visitor) addCall(from FQN, to FQN) {
	_, ok := v.calls[from]
	if !ok {
		v.calls[from] = make([]FQN, 0)
	}

	v.calls[from] = append(v.calls[from], to)
}

func (v *Visitor) fqn(r *ast.FuncDecl) string {
	name := r.Name.String()
	if r.Recv != nil {
		recv := r.Recv.List[0].Type
		name = fmt.Sprintf("%s:%s", v.fromExpr(recv), name)
	}

	return v.pkg.String() + "." + name
}

func (v *Visitor) fromExpr(e ast.Expr) *ast.Ident {
	switch exp := e.(type) {
	case *ast.StarExpr:
		return v.fromExpr(exp.X)
	case *ast.SelectorExpr:
		return exp.Sel
	case *ast.Ident:
		return exp
	default:
		fmt.Println("OTHER: ", reflect.TypeOf(exp))
	}

	return nil
}

func (v *Visitor) Sequences() map[FQN]*Sequences {
	return v.sequences
}

func (v *Visitor) Calls() map[FQN][]FQN {
	return v.calls
}

func (v *Visitor) analyzeBody(fqn FQN, body *ast.BlockStmt) {
	sequences := NewSequences(v.program, v.pkg)

	for _, stmt := range body.List {
		sequences.Track(stmt)
	}

	sequences.EndBlock()

	if sequences.HasAnyScope() {
		v.sequences[fqn] = sequences
	}
}
