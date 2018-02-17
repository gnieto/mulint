package main

import (
	"go/ast"

	"github.com/gnieto/mulint/mulint"
)

func main() {
	p := mulint.Load()
	errors := make([]mulint.LintError, 0)

	for _, pkg := range p.AllPackages {
		v := mulint.NewVisitor(p, pkg)

		for _, file := range pkg.Files {
			ast.Walk(v, file)
		}

		a := mulint.NewAnalyzer(pkg, v.Sequences(), v.Calls())
		// TODO: Analyze should return errors, probablty. It does not make sense
		// call Analyze and then call to Errors to retrieve them...
		a.Analyze()
		errors = append(errors, a.Errors()...)
	}

	stdout := mulint.NewStdOutReporter(p)
	stdout.Report(errors)
}
