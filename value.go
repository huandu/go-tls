// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package tls

import (
	"io"
)

// Data holds a value in TLS.
type Data interface {
	io.Closer
	Value() interface{}
}

type dataImpl struct {
	value  interface{}
	closer io.Closer
}

func (d *dataImpl) Value() interface{} {
	return d.value
}

func (d *dataImpl) Close() error {
	if d.closer == nil {
		return nil
	}

	closer := d.closer
	d.closer = nil
	return closer.Close()
}

// MakeData wraps data to Data.
// If data implements io.Closer, Data#Close will call data.Close().
func MakeData(data interface{}) Data {
	d := &dataImpl{
		value: data,
	}

	if closer, ok := data.(io.Closer); ok {
		d.closer = closer
	}

	return d
}
