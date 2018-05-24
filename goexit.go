// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

// +build go1.7

package tls

import (
	"runtime"
	"unsafe"
)

var (
	hackedGoexitFn   uintptr
	originalGoexitFn uintptr
)

const (
	funcSymbolSize = unsafe.Sizeof(_func{})
)

// interfaceImpl is the underlying data structure of an interface{} of func.
type interfaceImpl struct {
	typ     unsafe.Pointer
	funcPtr *uintptr
}

// stack is the stack position of a goroutine.
//
// The first field of type g, which is defined in package "runtime", is the stack struct.
// This doesn't change for several years since Go 1.5.
// I guess I can rely on it to read stack position.
type stack struct {
	lo uintptr
	hi uintptr
}

// Get goexit pc.
func init() {
	ch := make(chan uintptr, 1)
	go func() {
		pc := make([]uintptr, 16)
		sz := runtime.Callers(0, pc)
		ch <- pc[sz-1]
	}()
	originalGoexitFn = <-ch

}

// Get hacked goexit pc.
func init() {
	entry := runtime.FuncForPC(originalGoexitFn).Entry()
	pcQuantum := originalGoexitFn - entry

	var hackedIf interface{} = hackedGoexit
	hackedGoexitFn = *(*interfaceImpl)(unsafe.Pointer(&hackedIf)).funcPtr + pcQuantum

	fnHacked := runtime.FuncForPC(hackedGoexitFn)
	fnSymtab := (*_func)(unsafe.Pointer(fnHacked))

	// Start to hack func symtab.
	mprotect(unsafe.Pointer(fnSymtab), funcSymbolSize, protectWrite)
	fnSymtab.pcsp = 0

	// Restore symtab protect.
	mprotect(unsafe.Pointer(fnSymtab), funcSymbolSize, protectRead)
}

func hackedGoexit()

func hackedGoexit1() {
	resetAtExit()
	runtime.Goexit()
	panic("never return")
}

const align = 4

func hack(gp unsafe.Pointer) (success bool) {
	return swapGoexit(gp, originalGoexitFn, hackedGoexitFn)
}

func unhack(gp unsafe.Pointer) (success bool) {
	return swapGoexit(gp, hackedGoexitFn, originalGoexitFn)
}

func swapGoexit(gp unsafe.Pointer, from, to uintptr) (success bool) {
	s := (*stack)(gp)
	stackSize := (s.hi - uintptr(unsafe.Pointer(&gp))) &^ (1<<align - 1)
	start := s.hi - stackSize
	sp := (*(**[1000000]byte)(unsafe.Pointer(&start)))[:stackSize:stackSize]

	// Brute-force search goexit on stack.
	// We must find the last match to avoid any accidentally match.
	for offset := len(sp) - 8; offset >= 0; offset -= align {
		val := (*uintptr)(unsafe.Pointer(&sp[offset]))

		if *val == from {
			*val = to
			success = true
			break
		}
	}

	return
}
