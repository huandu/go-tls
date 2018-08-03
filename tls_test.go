// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package tls

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type tlsKey1 struct{}
type tlsKey2 struct{}
type tlsKey3 struct{}

type payload struct {
	data [1024]byte
}

func triggerMoreStack(n int) int {
	if n <= 0 {
		return 0
	}

	// Avoid tail optimization.
	return triggerMoreStack(n-1) + n
}

type closerFunc func()

func (f closerFunc) Close() error {
	f()
	return nil
}

func TestTLS(t *testing.T) {
	times := 1000
	idMap := map[int64]struct{}{}
	idMapMu := sync.Mutex{}

	for i := 0; i < times; i++ {
		t.Run(fmt.Sprintf("Round %v", i), func(t *testing.T) {
			closed := false
			k1 := tlsKey1{}
			v1 := 1234
			k2 := tlsKey2{}
			v2 := "v2"
			k3 := tlsKey3{}
			v3 := closerFunc(func() {
				closed = true
			})
			cnt := 0

			Set(k1, MakeData(v1))
			Set(k2, MakeData(v2))
			Set(k3, MakeData(v3))

			cnt++
			AtExit(func() {
				cnt--

				if expected := 0; cnt != expected {
					t.Fatalf("AtExit should call func in FILO order.")
				}
			})

			cnt++
			AtExit(func() {
				cnt--

				if expected := 1; cnt != expected {
					t.Fatalf("AtExit should call func in FILO order.")
				}
			})

			if d, ok := Get(k1); !ok || d == nil || !reflect.DeepEqual(d.Value(), v1) {
				t.Fatalf("fail to get k1.")
			}

			if d, ok := Get(k2); !ok || d == nil || !reflect.DeepEqual(d.Value(), v2) {
				t.Fatalf("fail to get k2.")
			}

			triggerMoreStack(10)

			Reset()

			if !closed {
				t.Fatalf("v3.Close() is not called.")
			}

			if _, ok := Get(k1); ok {
				t.Fatalf("k1 should be empty.")
			}

			Set(k1, MakeData(v1))
			Set(k1, MakeData(v2))

			if d, ok := Get(k1); !ok || d == nil || !reflect.DeepEqual(d.Value(), v2) {
				t.Fatalf("fail to get k1.")
			}

			if _, ok := Get(k2); ok {
				t.Fatalf("k2 should be empty.")
			}

			cnt++
			AtExit(func() {
				cnt--

				if expected := 2; cnt != expected {
					t.Fatalf("AtExit should call func in FILO order.")
				}
			})

			id := ID()

			if id <= 0 {
				t.Fatalf("fail to get ID. [id:%v]", id)
			}

			idMapMu.Lock()
			defer idMapMu.Unlock()

			if _, ok := idMap[id]; ok {
				t.Fatalf("duplicated ID. [id:%v]", id)
			}

			idMap[id] = struct{}{}
		})
	}
}

func TestUnload(t *testing.T) {
	// Run test in a standalone goroutine.
	t.Run("try unload", func(t *testing.T) {
		id := ID()
		exitCalled := false
		AtExit(func() {
			exitCalled = true
		})
		key := "key"
		expected := "value"
		Set(key, MakeData(expected))

		if d, ok := Get(key); !ok {
			t.Fatalf("fail to get data. [key:%v]", key)
		} else if actual, ok := d.Value().(string); !ok || actual != expected {
			t.Fatalf("invalid value. [key:%v] [value:%v] [expected:%v]", key, actual, expected)
		}

		Unload()

		// It's ok to call it again.
		Unload()

		if id == ID() {
			t.Fatalf("id must be changed after unload. [id:%v]", id)
		}

		if _, ok := Get(key); ok {
			t.Fatalf("key must be cleared. [key:%v]", key)
		}

		if exitCalled {
			t.Fatalf("all AtExit functions must not be called.")
		}
	})
}
func TestShrinkStack(t *testing.T) {
	const times = 20000
	const gcTimes = 100
	sleep := 10 * time.Millisecond
	errors := make(chan error, times)
	var done int64

	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup
	wg.Add(times)

	for i := 0; i < times; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errors <- fmt.Errorf("recovered with message: %v", r)
				}
			}()

			AtExit(func() {
				atomic.AddInt64(&done, 1)
				wg.Done()
			})
			n := rand.Intn(gcTimes)

			for j := 0; j < n; j++ {
				triggerMoreStack(100)
				time.Sleep(time.Duration((0.5 + rand.Float64()) * float64(sleep)))
			}
		}()
	}

	exit := make(chan bool, 2)
	go func() {
		wg.Wait()
		exit <- true
	}()

	go func() {
		// Avoid deadloop.
		select {
		case <-time.After(60 * time.Second):
			exit <- false
		}
	}()

GC:
	for {
		time.Sleep(sleep)
		runtime.GC()

		select {
		case <-exit:
			break GC
		default:
		}
	}

	close(errors)
	failed := false

	for err := range errors {
		failed = true
		t.Logf("panic [err:%v]", err)
	}

	if failed {
		t.FailNow()
	}

	if done != times {
		t.Fatalf("some AtExit handlers are not called. [expected:%v] [actual:%v]", times, done)
	}
}
