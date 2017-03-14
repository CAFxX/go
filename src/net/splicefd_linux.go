package net

import (
	"io"
	"syscall"
)

func splicefd(outFD *netFD, r io.Reader) (int64, error, bool) {
	var remain int64 = 1 << 62 // by default, copy until EOF

	if lr, ok := r.(*io.LimitedReader); ok {
		remain, r = lr.N, lr.R
		if remain <= 0 {
			return 0, nil, true
		}
	}
	if f, ok := r.(*TCPConn); ok {
		return _splicefd(f.fd, outFD, remain)
	}
	return 0, nil, false
}

func _splicefd(inFD, outFD *netFD, max int64) (int64, error, bool) {
	var pFD [2]int
	if err := syscall.Pipe(pFD[:]); err != nil {
		return 0, err, false
	}
	defer func() {
		syscall.Close(pFD[1])
		syscall.Close(pFD[0])
	}()

	for unread := max; unread > 0; {
		maxr := 1<<31 - 1 // MaxInt32
		if unread < int64(maxr) {
			maxr = int(unread)
		}
		nr, err := syscall.Splice(inFD.pfd.Sysfd, nil, pFD[1], nil, maxr, 1)
		if err != nil || nr == 0 {
			return max - unread, err, max != unread
		}
		unread -= int64(nr)
		for nr > 0 {
			nw, err := syscall.Splice(pFD[0], nil, outFD.pfd.Sysfd, nil, int(nr), 5)
			if err != nil {
				return max - unread, err, true
			}
			nr -= nw
		}
	}

	return max, nil, true
}
