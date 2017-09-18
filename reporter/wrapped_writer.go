// Copyright 2017 Mark Nevill. All Rights Reserved.
// See LICENSE for licensing terms.

package http_reporter

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
)

type wrappedWriter interface {
	http.ResponseWriter
	Status() int
	Size() int
}

func wrapWriter(w http.ResponseWriter, started func(int), hijacked func()) wrappedWriter {
	wrapped := &writer{
		parent:   w,
		started:  started,
		hijacked: hijacked,
	}

	c, isCloseNotifier := w.(http.CloseNotifier)
	f, isFlusher := w.(http.Flusher)
	h, isHijacker := w.(http.Hijacker)
	p, isPusher := w.(http.Pusher)
	rf, isReaderFrom := w.(io.ReaderFrom)

	// Check for the two most common combination of interfaces an http.ResponseWriter might implement.
	if !isHijacker && isPusher && isCloseNotifier {
		// http2.responseWriter (http 2.0)
		return &writerHTTP2{writer: wrapped, c: c, h: h, p: p}
	} else if isCloseNotifier && isFlusher && isHijacker && isReaderFrom {
		// http.response (http 1.1)
		return &writerHTTP1{writer: wrapped, c: c, f: f, h: h, rf: rf}
	}

	fmt.Println("Warn: Found not supported combination of extra http.ResponseWriter " +
		"interfaces. This combination differs from standard HTTP 2.0 or HTTP 1.1. Plain http.ResponseWriter wrapper will returned.")
	return wrapped
}

type writer struct {
	parent   http.ResponseWriter
	started  func(int)
	hijacked func()
	status   int
	size     int
}

func (w *writer) Status() int {
	return w.status
}

func (w *writer) Size() int {
	return w.size
}

func (w *writer) Header() http.Header {
	return w.parent.Header()
}

func (w *writer) WriteHeader(status int) {
	if w.started != nil {
		w.status = status
		w.started(status)
		w.started = nil
	}
	w.parent.WriteHeader(status)
}

func (w *writer) Write(buf []byte) (int, error) {
	if w.started != nil {
		w.status = http.StatusOK
		w.started(w.status)
		w.started = nil
	}
	n, err := w.parent.Write(buf)
	w.size += n
	return n, err
}

type writerHTTP2 struct {
	*writer
	c http.CloseNotifier
	h http.Hijacker
	p http.Pusher
}

func (w writerHTTP2) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.hijacked != nil {
		w.hijacked()
		w.hijacked = nil
	}
	return w.h.Hijack()
}

func (w writerHTTP2) CloseNotify() <-chan bool {
	return w.c.CloseNotify()
}

func (w writerHTTP2) Push(target string, opts *http.PushOptions) error {
	return w.p.Push(target, opts)
}

type writerHTTP1 struct {
	*writer
	c  http.CloseNotifier
	f  http.Flusher
	h  http.Hijacker
	rf io.ReaderFrom
}

func (w writerHTTP1) CloseNotify() <-chan bool {
	return w.c.CloseNotify()
}

func (w writerHTTP1) Flush() {
	w.f.Flush()
}

func (w writerHTTP1) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.hijacked != nil {
		w.hijacked()
		w.hijacked = nil
	}
	return w.h.Hijack()
}

func (w writerHTTP1) ReadFrom(r io.Reader) (int64, error) {
	n, err := w.rf.ReadFrom(r)
	w.size += int(n)
	return n, err
}
