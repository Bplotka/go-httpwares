// Copyright 2017 Mark Nevill. All Rights Reserved.
// See LICENSE for licensing terms.

package http_reporter_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/mwitkow/go-httpwares"
	"github.com/mwitkow/go-httpwares/prometheus"
	"github.com/mwitkow/go-httpwares/reporter"
	"github.com/mwitkow/go-httpwares/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testReporter struct {
	tracked int

	reqstarted  int
	reqread     int
	respstarted int
	respdone    int

	reqsize  int
	status   int
	respsize int
}

func (r *testReporter) Track(req *http.Request) http_reporter.Tracker {
	r.tracked += 1
	return r
}

func (r *testReporter) RequestStarted() {
	r.reqstarted += 1
}

func (r *testReporter) RequestRead(duration time.Duration, size int) {
	r.reqread += 1
	r.reqsize = size
}

func (r *testReporter) ResponseStarted(duration time.Duration, status int, header http.Header) {
	r.respstarted += 1
	r.status = status
}

func (r *testReporter) ResponseDone(duration time.Duration, status int, size int) {
	r.respdone += 1
	r.respsize = size
}

func handler(t *testing.T, status int, req string, resp string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, req, string(b))
		r.Body.Close()
		w.WriteHeader(status)
		w.Write([]byte(resp))
	})
}

func TestMiddleware_ReportsAllStats(t *testing.T) {
	r := &testReporter{}
	s := httptest.NewServer(chi.Chain(http_reporter.Middleware(r)).Handler(handler(t, 200, "req-body", "resp-body")))
	defer s.Close()
	resp, err := http.Post(s.URL, "text/plain", bytes.NewBufferString("req-body"))
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, &testReporter{
		tracked:     1,
		reqstarted:  1,
		reqread:     1,
		respstarted: 1,
		respdone:    1,
		reqsize:     8,
		respsize:    9,
		status:      200,
	}, r)
}

func TestTripperware_ReportsAllStats(t *testing.T) {
	r := &testReporter{}
	s := httptest.NewServer(handler(t, 200, "req-body", "resp-body"))
	defer s.Close()
	c := httpwares.WrapClient(http.DefaultClient, http_reporter.Tripperware(r))
	resp, err := c.Post(s.URL, "test/plain", bytes.NewBufferString("req-body"))
	require.NoError(t, err)
	_, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, &testReporter{
		tracked:     1,
		reqstarted:  1,
		reqread:     1,
		respstarted: 1,
		respdone:    1,
		reqsize:     8,
		respsize:    9,
		status:      200,
	}, r)
}

func ExampleMiddleware() {
	r := chi.NewRouter()
	r.Use(http_ctxtags.Middleware("default"))
	r.Use(http_prometheus.Middleware(http_prometheus.WithLatency()))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	http.ListenAndServe(":8888", r)
}

func ExampleTripperware() {
	c := httpwares.WrapClient(
		http.DefaultClient,
		http_ctxtags.Tripperware(),
		http_prometheus.Tripperware(http_prometheus.WithName("testclient")),
	)
	c.Get("example.org/foo")
}

type testReporterWithHijack struct {
	testReporter
	hijacked int
}

func (r *testReporterWithHijack) Track(req *http.Request) http_reporter.Tracker {
	r.tracked += 1
	return r
}

func (r *testReporterWithHijack) ConnHijacked(duration time.Duration) {
	r.hijacked += 1
}

func handlerHijack(t *testing.T, req string, resp string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, req, string(b))
		r.Body.Close()
		h, ok := w.(http.Hijacker)
		require.True(t, ok, "http.ResponseWriter need to implement http.Hijacker for this test")

		conn, buff, err := h.Hijack()
		require.NoError(t, err)

		defer conn.Close()

		// Now we are speaking raw TCP, so put HTTP response bytes before actual response.
		buff.Write([]byte("HTTP/1.1 200 OK\r\n"))
		buff.Write([]byte(fmt.Sprintf("Content-Length: %v\r\n", len(resp))))
		buff.Write([]byte("Content-Type: text/plain; charset=utf-8\r\n"))
		buff.Write([]byte("\r\n"))
		buff.Write([]byte(resp))
		buff.Flush()
	})
}

func TestMiddleware_ReportsStatsWithHijack(t *testing.T) {
	r := &testReporterWithHijack{}
	s := httptest.NewServer(chi.Chain(http_reporter.Middleware(r)).Handler(
		handlerHijack(t, "req-body", "resp-body")),
	)
	defer s.Close()
	resp, err := http.Post(s.URL, "text/plain", bytes.NewBufferString("req-body"))
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, &testReporterWithHijack{
		testReporter: testReporter{
			tracked:     1,
			reqstarted:  1,
			reqread:     1,
			respstarted: 0,
			respdone:    0,
			reqsize:     8,
			respsize:    0,
			status:      0,
		},
		hijacked: 1,
	}, r)
}
