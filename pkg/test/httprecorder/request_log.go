package httprecorder

import (
	"net/http"
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

func (l *RequestLog) ReplaceURLPrefix(old, new string) {
	for i := range l.Requests {
		r := &l.Requests[i]
		if strings.HasPrefix(r.URL, old) {
			r.URL = new + strings.TrimPrefix(r.URL, old)
		}
	}
}
