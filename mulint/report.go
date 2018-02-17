package mulint

import (
	"bufio"
	"fmt"
	"go/token"
	"os"
	"strings"

	"golang.org/x/tools/go/loader"
)

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

type Location struct {
	pos token.Pos
}

func NewLocation(pos token.Pos) Location {
	return Location{
		pos: pos,
	}
}

type Reporter interface {
	Report([]LintError)
}

type StdOut struct {
	program *loader.Program
}

func NewStdOutReporter(p *loader.Program) *StdOut {
	return &StdOut{
		program: p,
	}
}

func (r *StdOut) Report(errors []LintError) {
	for _, e := range errors {
		secondLockPosition := r.program.Fset.Position(e.secondLock.pos)
		secondLockLine := r.getLine(secondLockPosition)
		originLockPosition := r.program.Fset.Position(e.origin.pos)
		originLine := r.getLine(originLockPosition)

		fmt.Printf("%s:[%d] Mutex lock is adquired on this line: %s\n", r.baseFilename(secondLockPosition.Filename), secondLockPosition.Line, strings.TrimSpace(secondLockLine))
		fmt.Printf("\t%s:[%d] But the same lock was acquired here: %s\n", r.baseFilename(originLockPosition.Filename), originLockPosition.Line, strings.TrimSpace(originLine))
	}
}

func (r *StdOut) getLine(position token.Position) string {
	lines := r.readfile(position.Filename)

	return lines[position.Line-1]
}

func (r *StdOut) baseFilename(filename string) string {
	parts := strings.Split(filename, "/")

	if len(parts) == 0 {
		return filename
	}

	return parts[len(parts)-1]
}

func (r *StdOut) readfile(filename string) []string {
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
