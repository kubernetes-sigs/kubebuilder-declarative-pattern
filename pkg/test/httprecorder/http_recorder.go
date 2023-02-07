package httprecorder

import (
	"bytes"
	"fmt"
	"io"
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
	entry := &LogEntry{}
	entry.Request = Request{
		Method: request.Method,
		URL:    request.URL.String(),
		Header: request.Header,
	}

	if request.Body != nil {
		requestBody, err := io.ReadAll(request.Body)
		if err != nil {
			panic("failed to read request body")
		}
		entry.Request.Body = string(requestBody)
		request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}

	streaming := false
	if request.URL.Query().Get("watch") == "true" {
		streaming = true
	}

	// We log the request here, because otherwise we miss long-running requests (watches)
	m.log.Entries = append(m.log.Entries, entry)

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

		if streaming {
			entry.Response.Body = "<streaming response not included>"
		} else if response.Body != nil {
			responseBody, err := io.ReadAll(response.Body)
			if err != nil {
				entry.Response.Body = fmt.Sprintf("<error reading response:%v>", err)
			} else {
				entry.Response.Body = string(responseBody)
				response.Body = io.NopCloser(bytes.NewReader(responseBody))
			}
		}
	}

	return response, err
}
