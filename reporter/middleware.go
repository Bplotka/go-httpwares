// Copyright 2017 Mark Nevill. All Rights Reserved.
// See LICENSE for licensing terms.

package http_reporter

import (
	"net/http"
	"time"

	"github.com/mwitkow/go-httpwares"
)

// Middleware returns a http.Handler middleware that sets up reporter callbacks.
// This middleware assumes HTTP/1.x-style requests/response behaviour. It will not work with servers that use
// hijacking, pushing, or other similar features.
func Middleware(reporter Reporter) httpwares.Middleware {
	return func(next http.Handler) http.Handler {
		if reporter == nil {
			return next
		}
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			tracker := reporter.Track(req)
			if tracker == nil {
				// No tracking.
				next.ServeHTTP(resp, req)
				return
			}

			start := time.Now()
			tracker.RequestStarted()
			req.Body = wrapBody(req.Body, func(size int) {
				tracker.RequestRead(time.Since(start), size)
			})

			wasHijacked := false
			wrapped := wrapWriter(resp, func(status int) {
				// WriteHeader or Write called for the first time.
				tracker.ResponseStarted(time.Since(start), status, resp.Header())
			}, func() {
				// Hijack called.
				if h, ok := tracker.(HijackedTracker); ok {
					h.ConnHijacked(time.Since(start))
				}
				wasHijacked = true
			})

			next.ServeHTTP(wrapped, req)
			if !wasHijacked {
				// It makes sense only for NOT hijacked connection.
				tracker.ResponseDone(time.Since(start), wrapped.Status(), wrapped.Size())
			}
		})
	}
}
