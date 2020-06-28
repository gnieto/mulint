package tests

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/gnieto/mulint/mulint"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test_MixedLocks(t *testing.T) {

	filemap := map[string]string{
		"tests/mixed_locks.go":     LoadFile("mixed_locks.go"),
		"tests/simple_rlock.go":    LoadFile("simple_rlock.go"),
		"tests/transitive_lock.go": LoadFile("transitive_lock.go"),
	}
	dir, cleanup, err := analysistest.WriteFiles(filemap)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	result := analysistest.Run(t, dir, mulint.Mulint, "tests")

	failure := false
	for _, r := range result {
		if r.Err != nil {
			fmt.Println(r.Err)
			failure = true
		}
	}

	if failure {
		t.Fail()
	}
}

func LoadFile(path string) string {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic("Error loading file: " + err.Error())
	}

	return string(contents)
}
