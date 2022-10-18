package httprecorder

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type HTTPRecorder struct {
	inner http.RoundTripper
	log   *RequestLog
}

func NewRecorder(inner http.RoundTripper, log *RequestLog) *HTTPRecorder {
	rt := &HTTPRecorder{inner: inner, log: log}
	return rt
}

func (m *HTTPRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	c := Request{
		Method: req.Method,
		URL:    req.URL.String(),
		Header: req.Header,
	}

	if req.Body != nil {
		requestBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic("failed to read request body")
		}
		c.Body = string(requestBody)
		req.Body = ioutil.NopCloser(bytes.NewReader(requestBody))
	}
	m.log.Requests = append(m.log.Requests, c)

	return m.inner.RoundTrip(req)
}
