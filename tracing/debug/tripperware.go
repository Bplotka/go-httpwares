// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package http_debug

import (
	"bytes"
	"fmt"
	"net/http"

	"time"

	"github.com/mwitkow/go-httpwares"
	"github.com/mwitkow/go-httpwares/reporter"
	"github.com/mwitkow/go-httpwares/tags"
	"golang.org/x/net/trace"
)

const (
	headerMaxLength = 100
)

func clientDebug(opts ...opt) http_reporter.Reporter {
	o := evalOpts(opts)
	return &clientReporter{opts: o}
}

// Tripperware returns a piece of client-side Tripperware that puts requests on the `/debug/requests` page.
//
// The data logged will be: request headers, request ctxtags, response headers and response length.
func Tripperware(opts ...opt) httpwares.Tripperware {
	return http_reporter.Tripperware(clientDebug(opts...))
}

type clientReporter struct {
	opts *options
}

func (r *clientReporter) Track(req *http.Request) http_reporter.Tracker {
	if r.opts.filterFunc != nil && !r.opts.filterFunc(req) {
		return nil
	}

	tr := trace.New(operationNameFromUrl(req), req.URL.String())
	tr.LazyPrintf("%v %v HTTP/%d.%d", req.Method, req.URL, req.ProtoMajor, req.ProtoMinor)
	tr.LazyPrintf("%s", fmtHeaders(req.Header))
	return &serverTracker{
		req:  req,
		opts: r.opts,
		tr:   tr,
	}
}

type clientTracker struct {
	req  *http.Request
	opts *options
	tr   trace.Trace

	// Filled in ResponseStarted call.
	header http.Header
}

func (t *clientTracker) RequestStarted() {}

func (t *clientTracker) RequestRead(duration time.Duration, size int) {}

func (t *clientTracker) ResponseStarted(duration time.Duration, code int, header http.Header) {
	t.tr.LazyPrintf("%s", fmtTags(http_ctxtags.ExtractInbound(t.req).Values()))
}

func (t *clientTracker) ResponseDone(duration time.Duration, code int, size int) {
	// TODO now:
	// This is bottleneck... Current interface methods could be too limited.. Whole Resp and error is needed here ):
	// THe same will be wil logging.
	defer t.tr.Finish()
	if code == 599 {
		t.tr.LazyPrintf("Error on response: %v", err)
		t.tr.SetError()
	} else {
		t.tr.LazyPrintf("HTTP/%d.%d %d %s", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.Status)
		t.tr.LazyPrintf("%s", fmtHeaders(resp.Header))
		if t.opts.statusCodeErrorFunc(code) {
			t.tr.SetError()
		}
	}
	t.tr.Finish()
}

func operationNameFromUrl(req *http.Request) string {
	if tags := http_ctxtags.ExtractOutbound(req); tags.Has(http_ctxtags.TagForCallService) {
		vals := tags.Values()
		return fmt.Sprintf("%v.%s", vals[http_ctxtags.TagForCallService], req.Method)
	}
	return fmt.Sprintf("%s%s", req.URL.Host, req.URL.Path)
}

func fmtTags(t map[string]interface{}) *bytes.Buffer {
	var b bytes.Buffer
	b.WriteString("tags:")
	for k, v := range t {
		fmt.Fprintf(&b, " %v=%q", k, v)
	}
	return &b
}

func fmtHeaders(h http.Header) *bytes.Buffer {
	var buf bytes.Buffer
	for k := range h {
		v := h.Get(k)
		l := buf.Len()
		if len(k) > headerMaxLength {
			k = k[:headerMaxLength]
		}
		if len(v) > headerMaxLength {
			v = v[:headerMaxLength]
		}
		fmt.Fprintf(&buf, "%v: %v", k, v)
		if buf.Len() > l+headerMaxLength {
			buf.Truncate(l + headerMaxLength)
			fmt.Fprint(&buf, " (header truncated)")
		}
		buf.WriteByte('\n')
	}
	if buf.Len() > 0 {
		buf.Truncate(buf.Len() - 1)
	}
	return &buf
}
