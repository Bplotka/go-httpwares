// Copyright 2017 Mark Nevill. All Rights Reserved.
// See LICENSE for licensing terms.

package http_reporter

import (
	"net/http"
	"time"
)

// Called when a new request is to be tracked.
type Reporter interface {
	// Start tracking a new request.
	Track(req *http.Request) Tracker
}

// Receives events about a tracked request.
type Tracker interface {
	// The exchange has started. This is called immediately after Reporter.Track.
	// On the client, this is called before any data is sent.
	// On the server, this is called after headers have been parsed.
	RequestStarted()
	// The request body has been read to EOF or closed, whichever comes first.
	// On the client, this is called when the transport completes sending the request.
	// On the server, this is called when the handler completes reading the request, and may be omitted.
	RequestRead(duration time.Duration, size int)
	// The handling of the response has started.
	// On the client, this is called after the response headers have been parsed.
	// On the server, this is called before any data is written. In case of hijacked connection this may be omitted.
	ResponseStarted(duration time.Duration, status int, header http.Header)
	// The response has completed.
	// On the client, this is called when the body is read to EOF or closed, whichever comes first, and may be omitted.
	// On the server, this is called when the handler returns and has therefore completed writing the response. It may
	// be omitted when connection was hijacked.
	ResponseDone(duration time.Duration, status int, size int)
}

// Receives events about a hijacked request.
type HijackedTracker interface {
	// The connection was hijacked.
	// This can only be called on server when responseWriter's Hijack method is called.
	// Note that ResponseStarted can be still called if the hijack happen after the first write.
	// ResponseDone will omitted intentionally, because we don't have the control when the response will be actually
	// done on hijacked connection.
	ConnHijacked(duration time.Duration)
}
