// +build !linux

package net

import "io"

func splicefd(_ *netFD, _ io.Reader) (int64, error, bool) {
	return 0, nil, false
}
