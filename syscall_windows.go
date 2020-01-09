// +build windows

package tls

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

const (
	protectRead  = windows.PAGE_READONLY
	protectWrite = windows.PAGE_READWRITE
)

func mprotect(ptr unsafe.Pointer, size, prot uintptr) {
	var oldprotect uint32
	err := windows.VirtualProtect(uintptr(ptr), size, uint32(prot), &oldprotect)
	if err != nil {
		panic(fmt.Errorf("tls: fail to call VirtualProtect(addr=0x%x, size=%v, prot=0x%x) with error %v", ptr, size, prot, err))
	}
}
