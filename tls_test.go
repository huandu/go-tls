// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package tls

import (
	"fmt"
	"reflect"
	"testing"
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
	times := 100

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
		})
	}
}
