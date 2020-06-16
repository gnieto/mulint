package mulint

import (
	"bufio"
	"go/token"
	"os"
	"strings"

	"golang.org/x/tools/go/analysis"
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

func (le LintError) Origin() Location {
	return le.origin
}

func (le LintError) SecondLock() Location {
	return le.secondLock
}

func (le LintError) Report(pass *analysis.Pass) {
	secondLockPosition := pass.Fset.Position(le.secondLock.pos)
	secondLockLine := le.GetLine(pass, secondLockPosition)
	originLockPosition := pass.Fset.Position(le.origin.pos)
	originLine := le.GetLine(pass, originLockPosition)

	pass.Reportf(le.secondLock.Pos(),
		"Mutex lock is adquired on this line: %s\n\t%s:[%d] But the same lock was acquired here: %s\n",
		strings.TrimSpace(secondLockLine),
		le.baseFilename(originLockPosition.Filename),
		originLockPosition.Line,
		strings.TrimSpace(originLine),
	)
}

func (le LintError) GetLine(pass *analysis.Pass, position token.Position) string {
	lines := le.readfile(position.Filename)

	return lines[position.Line-1]
}

func (le LintError) baseFilename(filename string) string {
	parts := strings.Split(filename, "/")

	if len(parts) == 0 {
		return filename
	}

	return parts[len(parts)-1]
}

func (le LintError) readfile(filename string) []string {
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

type Location struct {
	pos token.Pos
}

func NewLocation(pos token.Pos) Location {
	return Location{
		pos: pos,
	}
}

func (l Location) Pos() token.Pos {
	return l.pos
}
