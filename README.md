# go-tls: TLS for any goroutine

[![Build Status](https://travis-ci.com/huandu/go-tls.svg?branch=master)](https://travis-ci.com/huandu/go-tls)
[![GoDoc](https://godoc.org/github.com/huandu/go-tls?status.svg)](https://pkg.go.dev/github.com/huandu/go-tls)

_WARNING: It's not recommended to use this package in any production environment. It may crash you at any time. Use `context` instead when possible._

Package `tls` provides TLS for any goroutine by hijacking `runtime.goexit` on stack. Comparing with other similar packages, this package avoids any potential resource leak in TLS.

## Install

Use `go get` to install this package.

```bash
    go get github.com/huandu/go-tls
```

## Use TLS

Set arbitrary data and get it later.

```go
k := "my key"
v := 1234
tls.Set(k, tls.MakeData(v))

// Get data by k.
d, ok := tls.Get(k)
assert(ok)
assert(d.Value().(int) == v)

// Get a unique ID for current goroutine.
// It's guaranteed to be unique.
id := tls.ID()

// Delete data by k.
tls.Del(k)

// Reset TLS so that all keys are removed and all data is closed if necessary.
// It doesn't remove any AtExit handler.
tls.Reset()

// Completely unload TLS and discard all data and AtExit handlers.
// If TLS method is called after Unload, a new TLS stub will be created.
// The ID() will return a different value.
tls.Unload()
```

If the data implements `io.Closer`, it will be called automatically when `Reset` is called or goroutine exits. It's not allowed to use any TLS methods in the `Close` method of TLS data. It will cause permanent memory leak.

## Execute code when goroutine exits

`AtExit` pushes a function to a slice of at-exit handlers and executes them when goroutine is exiting in FILO order. All TLS data is still available when calling at-exit handlers.

`AtExit` doesn't work on main goroutine as it doesn't exit at all.

```go
tls.AtExit(func() {
    // Do something when goroutine is exiting...
})
```

## Limitations

Several limitations so far.

- Works with Go 1.7 or newer.
- `AtExit` doesn't work on main goroutine, as this goroutine always exits with `os.Exit(0)` instead of calling `goexit`. See `main()` in `src/runtime/proc.go`.

## How it works

It's quite a long story I don't have time to write everything down right now.

TL; DR. Package `tls` uses goroutine's `g` struct pointer to identify a goroutine and hacks `runtime.goexit` to do house clean work when goroutine exits.

This approach is relatively safe, because all technics are based on runtime types which doesn't change since Go1.0.

Following runtime types are used.

- The `g.stack`: It's the first field of `g`. It stores stack memory range of a `g`.
- Function symbol table: When Go runtime allocates more stack, it validates all return addresses on stack. If I change `runtime.goexit` to another function pc, runtime will complain it as it's not a valid top of stack function (checked by `runtime.topofstack`). As a workaround, I hacks function symbol table to set `_func.pcsp` of the hacked goexit to `0` to skip checks.

## Similar packages

- [github.com/jtolds/gls](https://github.com/jtolds/gls): Goroutine local storage on current goroutine's stack. We must start goroutines with `Go` func explicitly before using any context methods.
- [github.com/v2pro/plz/gls](https://github.com/v2pro/plz/tree/master/gls): Use `goid` as a unique key for any goroutine and store contextual information.

## License

This package is licensed under MIT license. See LICENSE for details.
