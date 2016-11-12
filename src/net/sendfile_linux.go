// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"io"
	"os"
	"syscall"
)

// maxSendfileSize is the largest chunk size we ask the kernel to copy
// at a time.
const maxSendfileSize int = 4 << 20

// sendFile copies the contents of r to c using the sendfile
// system call to minimize copies.
//
// if handled == true, sendFile returns the number of bytes copied and any
// non-EOF error.
//
// if handled == false, sendFile performed no work.
func sendFile(c *netFD, r io.Reader) (written int64, err error, handled bool) {
	var remain int64 = 1 << 62 // by default, copy until EOF

	lr, ok := r.(*io.LimitedReader)
	if ok {
		remain, r = lr.N, lr.R
		if remain <= 0 {
			return 0, nil, true
		}
	}
	f, fok := r.(*os.File)
	s, sok := r.(*TCPConn)
	if !fok && !sok {
		return 0, nil, false
	}

	if err := c.writeLock(); err != nil {
		return 0, err, true
	}
	defer c.writeUnlock()

	if !fok && sok {
		// if sendfile can't be used (e.g. if r is a socket) do the splice dance
		read, written, err := splicefds(s.fd.sysfd, c.sysfd, remain)
		if lr != nil {
			lr.N = remain - read
		}
		return written, err, read > 0
	}

	dst := c.sysfd
	src := int(f.Fd())
	for remain > 0 {
		n := maxSendfileSize
		if int64(n) > remain {
			n = int(remain)
		}
		n, err1 := syscall.Sendfile(dst, src, nil, n)
		if n > 0 {
			written += int64(n)
			remain -= int64(n)
		}
		if n == 0 && err1 == nil {
			break
		}
		if err1 == syscall.EAGAIN {
			if err1 = c.pd.waitWrite(); err1 == nil {
				continue
			}
		}
		if err1 != nil {
			// This includes syscall.ENOSYS (no kernel
			// support) and syscall.EINVAL (fd types which
			// don't implement sendfile)
			err = err1
			break
		}
	}
	if lr != nil {
		lr.N = remain
	}
	if err != nil {
		err = os.NewSyscallError("sendfile", err)
	}
	return written, err, written > 0
}

func splicefds(in, out int, limit int64) (int64, int64, err) {
	const (
		SPLICE_F_MOVE     = 1
		SPLICE_F_NONBLOCK = 2
		SPLICE_F_MORE     = 4
		SPLICE_F_GIFT     = 8
		maxChunk          = 1 << 16 // bytes, TODO: fcntl(F_GETPIPE_SZ)?
	)

	var pipe [2]int
	err := syscall.Pipe(pipe)
	if err != nil {
		return 0, 0, os.NewSyscallError("pipe", err)
	}

	const spliceflags = SPLICE_F_MOVE | SPLICE_F_MORE
	read, written := int64(0), int64(0)

	for written < limit {
		chunk := maxChunk - (read - written)
		if limit-read < chunk {
			chunk = limit - read
		}

		n, err1 := syscall.Splice(src, nil, pipe[1], nil, chunk, spliceflags)
		if err1 != nil {
			err = os.NewSyscallError("splice", err1)
			break
		}
		read += int64(n)

		m, err1 := syscall.Splice(pipe[0], nil, dst, nil, read-written, spliceflags)
		if err1 != nil {
			err = os.NewSyscallError("splice", err1)
			break
		}
		if m == 0 {
			break
		}
		written += int64(m)
	}

	syscall.Close(pipe[1])
	syscall.Close(pipe[0])

	return read, written, err
}
