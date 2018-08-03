// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

// +build go1.7

// Package tls creates a TLS for a goroutine and release all resources at goroutine exit.
package tls

import (
	"io"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/huandu/go-tls/g"
)

var (
	tlsDataMap  = map[unsafe.Pointer]*tlsData{}
	tlsMu       sync.RWMutex
	tlsUniqueID int64
)

type tlsData struct {
	id          int64
	data        dataMap
	atExitFuncs []func()
}

type dataMap map[interface{}]Data

// As we cannot hack main goroutine safely,
// proactively create TLS for main to avoid hacking.
func init() {
	gp := g.G()

	if gp == nil {
		return
	}

	tlsMu.Lock()
	tlsDataMap[gp] = &tlsData{
		data: dataMap{},
	}
	tlsMu.Unlock()
}

// Get data by key.
func Get(key interface{}) (d Data, ok bool) {
	dm := fetchDataMap(true)

	if dm == nil {
		return
	}

	d, ok = dm.data[key]
	return
}

// Set data for key.
func Set(key interface{}, data Data) {
	dm := fetchDataMap(false)
	dm.data[key] = data
}

// Del data by key.
func Del(key interface{}) {
	dm := fetchDataMap(true)

	if dm == nil {
		return
	}

	delete(dm.data, key)
}

// ID returns a unique ID for a goroutine.
// If it's not possible to get the value, ID returns 0.
//
// It's guaranteed to be unique and consistent for one goroutine,
// unless it's called after Unload, which completely resets TLS stub.
// To be clear, it's not goid used by Go runtime.
func ID() int64 {
	dm := fetchDataMap(false)

	if dm == nil {
		return 0
	}

	return dm.id
}

// AtExit runs f when current goroutine is exiting.
// The f is called in FILO order.
func AtExit(f func()) {
	dm := fetchDataMap(false)
	dm.atExitFuncs = append(dm.atExitFuncs, f)
}

// Reset clears TLS data and releases all resources for current goroutine.
// It doesn't remove any AtExit handlers.
func Reset() {
	gp := g.G()

	if gp == nil {
		return
	}

	reset(gp, false)
}

func reset(gp unsafe.Pointer, complete bool) (alreadyReset bool) {
	var data dataMap

	tlsMu.Lock()
	dm := tlsDataMap[gp]

	if dm == nil {
		alreadyReset = true
	} else {
		data = dm.data

		if complete {
			delete(tlsDataMap, gp)
		} else {
			dm.data = dataMap{}
		}
	}

	tlsMu.Unlock()

	for _, d := range data {
		safeClose(d)
	}

	return
}

// Unload completely unloads TLS and clear all data and AtExit handlers.
func Unload() {
	gp := g.G()

	if gp == nil {
		return
	}

	if !reset(gp, true) {
		unhack(gp)
	}
}

func resetAtExit() {
	gp := g.G()

	if gp == nil {
		return
	}

	tlsMu.RLock()
	dm := tlsDataMap[gp]
	funcs := dm.atExitFuncs
	dm.atExitFuncs = nil
	tlsMu.RUnlock()

	// Call handlers in FILO order.
	for i := len(funcs) - 1; i >= 0; i-- {
		safeRun(funcs[i])
	}

	tlsMu.Lock()
	dm = tlsDataMap[gp]
	delete(tlsDataMap, gp)
	tlsMu.Unlock()

	for _, d := range dm.data {
		safeClose(d)
	}
}

// safeRun runs f and ignores any panic.
func safeRun(f func()) {
	defer func() {
		recover()
	}()
	f()
}

// safeClose closes closer and ignores any panic.
func safeClose(closer io.Closer) {
	defer func() {
		recover()
	}()
	closer.Close()
}

func fetchDataMap(readonly bool) *tlsData {
	gp := g.G()

	if gp == nil {
		return nil
	}

	// Try to find saved data.
	needHack := false
	tlsMu.RLock()
	dm := tlsDataMap[gp]
	tlsMu.RUnlock()

	if dm == nil && !readonly {
		needHack = true
		dm = &tlsData{
			id:   atomic.AddInt64(&tlsUniqueID, 1),
			data: dataMap{},
		}
		tlsMu.Lock()
		tlsDataMap[gp] = dm
		tlsMu.Unlock()
	}

	// Current goroutine is not hacked. Hack it.
	if needHack {
		if !hack(gp) {
			tlsMu.Lock()
			delete(tlsDataMap, gp)
			tlsMu.Unlock()
		}
	}

	return dm
}
