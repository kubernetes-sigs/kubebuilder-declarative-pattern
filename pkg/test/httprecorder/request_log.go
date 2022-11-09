package httprecorder

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

type LogEntry struct {
	Request  Request  `json:"request,omitempty"`
	Response Response `json:"response,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type Request struct {
	Method string      `json:"method,omitempty"`
	URL    string      `json:"url,omitempty"`
	Header http.Header `json:"header,omitempty"`
	Body   string      `json:"body,omitempty"`
}

type Response struct {
	Status     string      `json:"status,omitempty"`
	StatusCode int         `json:"statusCode,omitempty"`
	Header     http.Header `json:"header,omitempty"`
	Body       string      `json:"body,omitempty"`
}

func (e *LogEntry) FormatHTTP() string {
	var b strings.Builder
	b.WriteString(e.Request.FormatHTTP())
	b.WriteString(e.Response.FormatHTTP())
	return b.String()
}

func (r *Request) FormatHTTP() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s %s\n", r.Method, r.URL))
	var keys []string
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range r.Header[k] {
			b.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	b.WriteString("\n")
	if r.Body != "" {
		b.WriteString(r.Body)
		b.WriteString("\n\n")
	}
	return b.String()
}

func (r *Response) FormatHTTP() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s\n", r.Status))
	var keys []string
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range r.Header[k] {
			b.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	b.WriteString("\n")
	if r.Body != "" {
		b.WriteString(r.Body)
		b.WriteString("\n")
	}
	return b.String()
}

type RequestLog struct {
	Entries []*LogEntry
}

func (l *RequestLog) FormatHTTP() string {
	var actual []string
	for _, entry := range l.Entries {
		s := entry.FormatHTTP()
		actual = append(actual, s)
	}
	return strings.Join(actual, "\n---\n\n")
}

func (l *RequestLog) ReplaceURLPrefix(old, new string) {
	for i := range l.Entries {
		r := &l.Entries[i].Request
		if strings.HasPrefix(r.URL, old) {
			r.URL = new + strings.TrimPrefix(r.URL, old)
		}
	}
}

func (l *RequestLog) RemoveHeader(k string) {
	for i := range l.Entries {
		r := &l.Entries[i].Request
		r.Header.Del(k)
	}
}

func (l *RequestLog) RemoveUserAgent() {
	l.RemoveHeader("user-agent")
}

func (l *RequestLog) RegexReplaceURL(find string, replace string) {
	for i := range l.Entries {
		request := &l.Entries[i].Request
		u := request.URL
		r, err := regexp.Compile(find)
		if err != nil {
			klog.Fatalf("failed to compile regex %q: %v", find, err)
		}
		u = r.ReplaceAllString(u, replace)
		request.URL = u
	}
}
