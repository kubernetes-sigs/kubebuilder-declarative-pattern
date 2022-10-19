package httprecorder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type HTTPRecorder struct {
	inner http.RoundTripper
	log   *RequestLog
}

func NewRecorder(inner http.RoundTripper, log *RequestLog) *HTTPRecorder {
	rt := &HTTPRecorder{inner: inner, log: log}
	return rt
}

func (m *HTTPRecorder) RoundTrip(request *http.Request) (*http.Response, error) {
	entry := LogEntry{}
	entry.Request = Request{
		Method: request.Method,
		URL:    request.URL.String(),
		Header: request.Header,
	}

	if request.Body != nil {
		requestBody, err := ioutil.ReadAll(request.Body)
		if err != nil {
			panic("failed to read request body")
		}
		entry.Request.Body = string(requestBody)
		request.Body = ioutil.NopCloser(bytes.NewReader(requestBody))
	}

	response, err := m.inner.RoundTrip(request)

	if response != nil {
		entry.Response.Status = response.Status
		entry.Response.StatusCode = response.StatusCode

		entry.Response.Header = make(http.Header)
		for k, values := range response.Header {
			switch strings.ToLower(k) {
			case "authorization":
				entry.Response.Header[k] = []string{"(redacted)"}
			case "date":
				entry.Response.Header[k] = []string{"(removed)"}
			default:
				entry.Response.Header[k] = values
			}
		}

		if response.Body != nil {
			requestBody, err := ioutil.ReadAll(response.Body)
			if err != nil {
				entry.Response.Body = fmt.Sprintf("<error reading response:%v>", err)
			} else {
				entry.Response.Body = string(requestBody)
				response.Body = ioutil.NopCloser(bytes.NewReader(requestBody))
			}
		}
	}

	m.log.Entries = append(m.log.Entries, entry)

	return response, err
}
