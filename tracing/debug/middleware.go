// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package http_debug

import (
	"net/http"

	"fmt"

	"github.com/mwitkow/go-httpwares"
	"github.com/mwitkow/go-httpwares/tags"
	"golang.org/x/net/trace"
)

// Middleware returns a http.Handler middleware that writes inbound requests to /debug/request.
//
// The data logged will be: request headers, request ctxtags, response headers and response length.
func Middleware(opts ...Option) httpwares.Middleware {
	o := evaluateOptions(opts)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if o.filterFunc != nil && !o.filterFunc(req) {
				next.ServeHTTP(resp, req)
				return
			}
			tr := trace.New(operationNameFromReqHandler(req), req.RequestURI)
			defer tr.Finish()
			tr.LazyPrintf("%v %v HTTP/%d.%d", req.Method, req.RequestURI, req.ProtoMajor, req.ProtoMinor)
			tr.LazyPrintf("Host: %v", hostFromReq(req))
			for k, _ := range req.Header {
				tr.LazyPrintf("%v: %v", k, req.Header.Get(k))
			}
			tr.LazyPrintf("invoking next chain")
			newResp := httpwares.WrapResponseWriter(resp)
			next.ServeHTTP(newResp, req)
			tr.LazyPrintf("tags: ")
			for k, v := range http_ctxtags.ExtractInbound(req).Values() {
				tr.LazyPrintf("%v: %v", k, v)
			}
			tr.LazyPrintf("Response: %d", newResp.StatusCode())
			for k, _ := range resp.Header() {
				tr.LazyPrintf("%v: %v", k, resp.Header().Get(k))
			}
			if o.statusCodeErrorFunc(newResp.StatusCode()) {
				tr.SetError()
			}
		})
	}
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

func hostFromReq(req *http.Request) string {
	if req.URL.Path != "" {
		return req.URL.Path
	}
	return req.Host
}
