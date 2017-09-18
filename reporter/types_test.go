// Copyright 2017 Mark Nevill. All Rights Reserved.
// See LICENSE for licensing terms.

package http_reporter

import (
	"io"
	"net/http"
	"testing"
)

func TestWrappers_ImplementExpectedInterfaces(t *testing.T) {
	var _ io.WriterTo = bodyWT{}
	var _ http.CloseNotifier = writerHTTP1{}
	var _ http.Flusher = writerHTTP1{}
	var _ http.Hijacker = writerHTTP1{}
	var _ io.ReaderFrom = writerHTTP1{}

	var _ http.CloseNotifier = writerHTTP2{}
	var _ http.Hijacker = writerHTTP2{}
	var _ http.Pusher = writerHTTP2{}
}
