// +build !windows

/*
 * Copyright (C) 2017 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package dede

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
)

type terminalOpts struct {
	cols int
}

type terminal struct {
	sync.RWMutex
	cmd  string
	opts terminalOpts
	pty  *os.File
}

func newTerminal(cmd string, opts ...terminalOpts) *terminal {
	t := &terminal{
		cmd: cmd,
	}
	if len(opts) > 0 {
		t.opts = opts[0]
	}

	return t
}

type termsize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}

func setsize(f *os.File, ts *termsize) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(ts)),
	)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}

func (t *terminal) start(in chan []byte, out chan []byte, err chan error) {
	sz := &termsize{
		Cols: 80,
	}
	if t.opts.cols != 0 {
		sz.Cols = uint16(t.opts.cols)
	}

	p, e := pty.Start(exec.Command(t.cmd))
	if e != nil {
		err <- fmt.Errorf("failed to start: %s", e)
		return
	}
	setsize(p, sz)

	t.Lock()
	t.pty = p
	t.Unlock()

	// pty reading
	go func() {
		for {
			buf := make([]byte, 1024)
			n, e := p.Read(buf)
			data := buf[:n]

			if e != nil {
				err <- fmt.Errorf("failed to start: %s", e)
				return
			}
			out <- data
		}
	}()

	// pty writing
	go func() {
		for b := range in {
			if _, e := p.Write(b); e != nil {
				err <- fmt.Errorf("failed to start: %s", e)
				return
			}
		}
	}()
}

func (t *terminal) close() {
	if err := t.pty.Close(); err != nil {
		Log.Errorf("failed to stop: %s", err)
	}
}
