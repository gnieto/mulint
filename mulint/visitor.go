package mulint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/securego/gosec"
	"golang.org/x/tools/go/loader"
)

type MutexScope struct {
	mutexSelector string
	pos           token.Pos
	seq           []ast.Node
}

func NewMutexScope(mutexSelector string, pos token.Pos) *MutexScope {
	return &MutexScope{
		mutexSelector: mutexSelector,
		seq:           make([]ast.Node, 0),
		pos:           pos,
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

type Scopes struct {
	onGoing  map[string]*MutexScope
	defers   map[string]bool
	finished []*MutexScope
	prog     *loader.Program
}

func NewScopes(prog *loader.Program) *Scopes {
	return &Scopes{
		onGoing:  make(map[string]*MutexScope),
		defers:   make(map[string]bool),
		finished: make([]*MutexScope, 0),
		prog:     prog,
	}
}

func StrExpr(e ast.Expr) string {
	return fmt.Sprintf("%s", e)
}

func (s *Scopes) Track(stmt ast.Stmt) {
	for _, og := range s.onGoing {
		og.Add(stmt)
	}

	// Is start of a sequence to check?
	if e := s.isLockCall(stmt); e != nil {
		se := StrExpr(e)

		if _, exists := s.onGoing[se]; !exists {
			s.onGoing[se] = NewMutexScope(se, stmt.Pos())
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
			delete(s.onGoing, se)
		}
	}
}

func (s *Scopes) EndBlock() {
	for k := range s.defers {
		if og, ok := s.onGoing[k]; ok {
			s.finished = append(s.finished, og)
		}
	}

	s.onGoing = make(map[string]*MutexScope)
	s.defers = make(map[string]bool)
}

func (s *Scopes) HasAnyScope() bool {
	return len(s.finished) > 0
}

func (s *Scopes) Scopes() []*MutexScope {
	return s.finished
}

func (s *Scopes) isLockCall(node ast.Node) ast.Expr {
	return SubjectForCall(node, []string{"RLock", "Lock"})
}

func (s *Scopes) isUnlockCall(node ast.Node) ast.Expr {
	return SubjectForCall(node, []string{"RUnlock", "Unlock"})
}

func (s *Scopes) isDeferUnlockCall(node ast.Node) ast.Expr {
	switch sty := node.(type) {
	case *ast.DeferStmt:
		return s.isUnlockCall(sty.Call)
	default:
		return nil
	}
}

type Visitor struct {
	scopes  map[FQN]*Scopes
	calls   map[FQN][]FQN
	program *loader.Program
	pkg     *types.Package
	info    *types.Info
}

func NewVisitor(prog *loader.Program, pkg *types.Package, info *types.Info) *Visitor {
	return &Visitor{
		scopes:  make(map[FQN]*Scopes),
		calls:   make(map[FQN][]FQN),
		program: prog,
		pkg:     pkg,
		info:    info,
	}
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	switch stmt := node.(type) {
	case *ast.FuncDecl:
		body := stmt.Body
		fqn := FQN(v.fqn(stmt))

		// Protect if body is nil, which means that is a external non-go func
		if body == nil {
			return v
		}

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
			ctx := gosec.Context{
				Pkg:  v.pkg,
				Info: v.info,
			}

			pkg, name, err := gosec.GetCallInfo(call, &ctx)
			if err == nil {
				fqn := FromCallInfo(pkg, name)
				v.addCall(currentFQN, fqn)
			}
		}
	}
}

func (v *Visitor) addCall(from, to FQN) {
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
		name = fmt.Sprintf("%s:%s", v.extractFileAndFnName(recv), name)
	}

	return v.pkg.Path() + "." + name
}

func (v *Visitor) extractFileAndFnName(e ast.Expr) *ast.Ident {
	switch exp := e.(type) {
	case *ast.StarExpr:
		return v.extractFileAndFnName(exp.X)
	case *ast.SelectorExpr:
		return exp.Sel
	case *ast.Ident:
		return exp
	}

	return nil
}

func (v *Visitor) Scopes() map[FQN]*Scopes {
	return v.scopes
}

func (v *Visitor) Calls() map[FQN][]FQN {
	return v.calls
}

func (v *Visitor) analyzeBody(fqn FQN, body *ast.BlockStmt) {
	scopes := NewScopes(v.program)

	for _, stmt := range body.List {

		scopes.Track(stmt)
	}

	scopes.EndBlock()

	if scopes.HasAnyScope() {
		v.scopes[fqn] = scopes
	}
}
