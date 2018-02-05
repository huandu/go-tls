// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

// +build go1.7

// Package tls creates a TLS for a goroutine and release all resources at goroutine exit.
package tls

import (
	"io"
	"sync"
	"unsafe"

	"github.com/huandu/go-tls/g"
)

var (
	tlsDataMap = map[unsafe.Pointer]*tlsData{}
	tlsMu      sync.Mutex
)

type tlsData struct {
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

// AtExit runs f when current goroutine is exiting.
// The f is called in FILO order.
func AtExit(f func()) {
	dm := fetchDataMap(false)
	dm.atExitFuncs = append(dm.atExitFuncs, f)
}

// Reset clears TLS and releases all resources for current goroutine.
func Reset() {
	gp := g.G()

	if gp == nil {
		return
	}

	tlsMu.Lock()
	dm := tlsDataMap[gp]
	data := dm.data
	dm.data = dataMap{}
	tlsMu.Unlock()

	unhack(gp)

	for _, d := range data {
		safeClose(d)
	}
}

func resetAtExit() {
	gp := g.G()

	if gp == nil {
		return
	}

	tlsMu.Lock()
	dm := tlsDataMap[gp]
	funcs := make([]func(), 0, len(dm.atExitFuncs))
	funcs = append(funcs, dm.atExitFuncs...)
	tlsMu.Unlock()

	// Call handlers in FILO order.
	for i := len(funcs) - 1; i >= 0; i-- {
		safeRun(funcs[i])
	}

	tlsMu.Lock()
	dm = tlsDataMap[gp]
	tlsDataMap[gp] = nil
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
	tlsMu.Lock()
	dm := tlsDataMap[gp]

	if dm == nil && !readonly {
		needHack = true
		dm = &tlsData{
			data: dataMap{},
		}
		tlsDataMap[gp] = dm
	}

	tlsMu.Unlock()

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
