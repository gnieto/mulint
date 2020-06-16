package main

import (
	"github.com/gnieto/mulint/mulint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(mulint.Mulint)
}
