# The Go Programming Language - OmgSoManyKnobsMyHeadHurtsThankYou Edition

A clone/fork of the [Go compiler](https://golang.org) that gives users a few extra knobs to control compilation. Only useful for those times when the Go compiler doesn't get it right by itself.

## A word of caution

You probably don't need this. Really.

The standard Go compiler should be enough in 99.9% of cases. So unless what your trying to do falls in the remaining 0.1%, you should stop reading now and go back to work. If you don't know whether your case falls or not in the 0.1%, then it most certainly doesn't and you also should stop reading now and go back to work.

You have been warned.

The only good use of this is for very hot functions that can't be split into smaller, more readable and maintainable functions because of call overhead. Instead of playing pointless code golf with the compiler until each function fits inside its arbitrary inlining budget just mark them as `//go:yesinline`.

Note that all the limitations about what the compiler is actually able to inline still remain (e.g. no defer, closures, channels, for-loops, ...).

## Knobs

This fork is identical to the standard compiler, but it adds a couple of knobs in case you want a little more control over the [inliner](https://golang.org/src/cmd/compile/internal/gc/inl.go) behavior.

- `-inl-budget <budget>` gc compile option: allows to change the inlining budget (default: `80`)

  ```sh
  # set the inlining budget to 1000
  go build -gcflags="-inl-budget=1000"
  ```

- `-inl-leafonly <bool>` gc compile option: forces the the compiler to only inline leaf functions (default: `true`, WARNING: setting it to `false` is experimental, may break your code)

  ```sh
  # allow inlining non-leaf functions
  go build -gcflags="-inl-leafonly=false"
  ```

- `//go:yesinline` func pragma: hint that function should be inlined even if non-leaf or over inlining budget

  ```go
  //go:yesinline
  func FuncThatShouldBeInlined() {
    // ...
  }
  ```

## How to build

Make sure you have a recent Go installed and working. Then run:

```bash
git clone https://github.com/CAFxX/go
(cd go/src; GOROOT_BOOTSTRAP=$(go env GOROOT) ./all.bash)
# to use the built compiler, set:
# export GOROOT=$(pwd)/go
```

## TODO

- `//go:noinline` and `//go:yesinline` statement (function/method call) pragmas to give hints about whether to inline the (outermost) function/method call on the following line, e.g.

  ```go
  func baz(x int) int {
    // inhibit inlining of foo (the call to bar is not affected)
    //go:noinline
    return foo(bar(x))
  }
  ```

## License

My patch is in the public domain. No warranties of any kind.

Please refer to the LICENSE file for the license of the Go source code.
