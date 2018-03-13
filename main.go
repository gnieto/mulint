package main

import (
	"flag"
	"go/ast"
	"os"

	"github.com/gnieto/mulint/mulint"
	"github.com/kisielk/gotool"
)

func main() {
	errors := make([]mulint.LintError, 0)
	flag.Parse()
	var packages []string

	for _, pkg := range gotool.ImportPaths(flag.Args()) {
		packages = append(packages, pkg)
	}

	p := mulint.Load(packages)
	for _, pkg := range p.AllPackages {
		v := mulint.NewVisitor(p, pkg)

		for _, file := range pkg.Files {
			ast.Walk(v, file)
		}

		a := mulint.NewAnalyzer(pkg, v.Scopes(), v.Calls())
		// TODO: Analyze should return errors, probablty. It does not make sense
		// call Analyze and then call to Errors to retrieve them...
		a.Analyze()
		errors = append(errors, a.Errors()...)
	}

	stdout := mulint.NewStdOutReporter(p)
	stdout.Report(errors)

	if len(errors) > 0 {
		os.Exit(1)
	}
}
