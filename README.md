# mulint

mulint is a program/lint to check potential dead locks on go programs. The documentation for `sync.RWMutex` says:

```
If a goroutine holds a RWMutex for reading and another goroutine might call Lock, no goroutine should expect to be able to acquire a read lock until the initial read lock is released. In particular, this prohibits recursive read locking. This is to ensure that the lock eventually becomes available; a blocked Lock call excludes new readers from acquiring the lock.
```

As this is stated on the documentation, but is not enforced by the language nor the library, this program tries to detect some recursive read locks on the same goroutine.

## Install

As usual on Go environment, to install, run the following command:

```
go get -u github.com/gnieto/mulint
```

This should install the lint on your `$GOPATH`.

## Running

To run the lint, you just need to:

```
mulint ./...
```

Note that this lint uses `golang.org/x/tools`, so all the usual wildcards to filter or select packages should work.

On the following lines, you can see an example of output of this lint:

```
mixed_locks.go:[27] Mutex lock is adquired on this line: m.Test()
	mixed_locks.go:[24] But the same lock was acquired here: m.m.Lock()
```

The first line indicates where the recursive lock is adquired (so, the second `Lock` on the same goroutine), the file and line where it occurs. The second one, indicate where the first `Lock` occurs

## Exit code

If the program detects some error, it will return a 1 as error code

## Limitations

At the moment, it can only detect some basic scenarios when a `Mutex` or `RWMutex` is part of a struct and all the `Lock` and `Unlock` operations are executed inside the scopes of the mentioned struct.
So, it may detect a recursive call like the following one:

```go
type someStruct struct {
  m sync.RWMutex
}

func (s *someStruct) A() {
  s.m.RLock()
  s.B()
  s.m.RUnlock()
}

func (s *someStruct) B() {
  s.m.RLock() // This is a recursive lock and it should be detected by this tool
  s.m.RUnlock()
}
```

But it's not able to detect a recursive lock when a mutex is moved to another scope, as it may happen with `func`s

```go
var m sync.RWMutex

A(m)

func A(m *sync.RWMutex) {
  m.RLock()
  B(m)
  m.RUnlock()
}

func B(m *sync.RWMutex) {
  m.RLock()
  m.RUnlock()
}
```

There is no techincal limitation (as far as I am aware) to track when a mutex is moved, but I left this cases for now. Feel free to do the change and open a PR to include this case.

Finally, all the analysis is done per package. If there's some recursive `RLock` which operates among distinct packages, it won't be detected by this lint.

## Attribution

- I have checked the `errcheck` code (https://github.com/kisielk/errcheck) as an example of how the AST is walked on Go
- Lint uses GoASTScanner (https://github.com/GoASTScanner/gas), just for a method that refused to re-writte myself (`GetCallInfo`)
