// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package tls

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	pageSize = 4096
)

const (
	protectNone  = syscall.PROT_NONE
	protectRead  = syscall.PROT_READ
	protectWrite = syscall.PROT_WRITE
	protectExec  = syscall.PROT_EXEC
)

var (
	goexitCode = make([]byte, pageSize*2)
)

func mprotect(ptr unsafe.Pointer, size, prot uintptr) {
	addr := uintptr(ptr)
	aligned := addr &^ (pageSize - 1)
	_, _, errno := syscall.Syscall(syscall.SYS_MPROTECT, aligned, addr-aligned+size, prot)

	if errno != 0 {
		panic(fmt.Errorf("tls: fail to call mprotect(addr=0x%x, size=%v, prot=0x%x) with error %v", addr, size, prot, errno))
	}
}
