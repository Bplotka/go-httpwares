// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package http_debug

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mwitkow/go-httpwares"
	"github.com/mwitkow/go-httpwares/reporter"
	"github.com/mwitkow/go-httpwares/tags"
	"golang.org/x/net/trace"
)

func serverDebug(opts ...opt) http_reporter.Reporter {
	o := evalOpts(opts)
	return &serverReporter{opts: o}
}

// Middleware returns a http.Handler middleware that writes inbound requests to /debug/request.
//
// The data logged will be: request headers, request ctxtags, response headers and response length.
func Middleware(opts ...opt) httpwares.Middleware {
	return http_reporter.Middleware(serverDebug(opts...))
}

type serverReporter struct {
	opts *options
}

func (r *serverReporter) Track(req *http.Request) http_reporter.Tracker {
	if r.opts.filterFunc != nil && !r.opts.filterFunc(req) {
		return nil
	}

	tr := trace.New(operationNameFromReqHandler(req), req.RequestURI)
	tr.LazyPrintf("%v %v HTTP/%d.%d", req.Method, req.RequestURI, req.ProtoMajor, req.ProtoMinor)
	tr.LazyPrintf("%s", fmtHeaders(req.Header))
	tr.LazyPrintf("invoking next chain")
	return &serverTracker{
		req:  req,
		opts: r.opts,
		tr:   tr,
	}
}

type serverTracker struct {
	req  *http.Request
	opts *options
	tr   trace.Trace

	// Filled in ResponseStarted call.
	header http.Header
}

func (t *serverTracker) RequestStarted() {}

func (t *serverTracker) RequestRead(duration time.Duration, size int) {}

func (t *serverTracker) ResponseStarted(duration time.Duration, code int, header http.Header) {
	t.header = header
}

func (t *serverTracker) ResponseDone(duration time.Duration, code int, size int) {
	t.tr.LazyPrintf("tags: ")
	for k, v := range http_ctxtags.ExtractInbound(t.req).Values() {
		t.tr.LazyPrintf("%v: %v", k, v)
	}
	t.tr.LazyPrintf("Response: %d", code)
	t.tr.LazyPrintf("%s", fmtHeaders(t.header))
	if t.opts.statusCodeErrorFunc(code) {
		t.tr.SetError()
	}
	t.tr.Finish()
}

func (t *serverTracker) ConnHijacked(duration time.Duration) {
	t.tr.LazyPrintf("tags: ")
	for k, v := range http_ctxtags.ExtractInbound(t.req).Values() {
		t.tr.LazyPrintf("%v: %v", k, v)
	}
	t.tr.LazyPrintf("Response: Unknown (hijacked)")
	t.tr.Finish()
}

func operationNameFromReqHandler(req *http.Request) string {
	if tags := http_ctxtags.ExtractInbound(req); tags.Has(http_ctxtags.TagForHandlerGroup) {
		vals := tags.Values()
		method := "unknown"
		if val, ok := vals[http_ctxtags.TagForHandlerName].(string); ok {
			method = val
		}
		return fmt.Sprintf("http.Recv.%v.%s", vals[http_ctxtags.TagForHandlerGroup], method)
	}
	return fmt.Sprintf("http.Recv.%s", req.URL.Path)
}
