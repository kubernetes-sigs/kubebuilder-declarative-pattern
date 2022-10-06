package httprecorder

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type Request struct {
	Method string      `json:"method,omitempty"`
	URL    string      `json:"url,omitempty"`
	Header http.Header `json:"header,omitempty"`
	Body   string      `json:"body,omitempty"`
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
		b.WriteString("\n")
	}
	return b.String()
}

type RequestLog struct {
	Requests []Request
}

func (l *RequestLog) FormatYAML() string {
	var actual []string
	for _, request := range l.Requests {
		y, err := yaml.Marshal(request)
		if err != nil {
			klog.Fatalf("error from yaml.Marshal: %v", err)
		}
		actual = append(actual, string(y))
	}
	return strings.Join(actual, "\n---\n")
}

func (l *RequestLog) FormatHTTP() string {
	var actual []string
	for _, request := range l.Requests {
		s := request.FormatHTTP()
		actual = append(actual, s)
	}
	return strings.Join(actual, "\n---\n\n")
}

func (l *RequestLog) ReplaceURLPrefix(old, new string) {
	for i := range l.Requests {
		r := &l.Requests[i]
		if strings.HasPrefix(r.URL, old) {
			r.URL = new + strings.TrimPrefix(r.URL, old)
		}
	}
}

func (l *RequestLog) RemoveUserAgent() {
	for i := range l.Requests {
		r := &l.Requests[i]
		r.Header.Del("user-agent")
	}
}
