package httprecorder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver"
	"sigs.k8s.io/yaml"
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

func (l *RequestLog) ReplaceTimestamp() {
	for _, entry := range l.Entries {
		entry.Request.Body = resetTimestamp(entry.Request.Body)
		entry.Response.Body = resetTimestamp(entry.Response.Body)
	}
}

func resetTimestamp(body string) string {
	if body == "" {
		return body
	}
	var u *unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(body), &u); err != nil {
		return body
	}

	if u.Object["status"] == nil {
		return body
	}
	status := u.Object["status"].(map[string]interface{})
	if status["conditions"] == nil {
		return body
	}
	conditions := status["conditions"].([]interface{})
	for _, condition := range conditions {
		cond := condition.(map[string]interface{})
		// mockkubeapiserver provides a mock timestamp.
		cond["lastTransitionTime"] = mockkubeapiserver.NewTestClock().Now().Format("2006-01-02T15:04:05Z07:00")
	}
	b, _ := json.Marshal(u)
	return string(b)
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

// SortGETs attempts to normalize parallel requests.
// Consecutive GET requests are sorted alphabetically.
func (l *RequestLog) SortGETs() {

	isSwappable := func(urlString string) (string, bool) {
		u, err := url.Parse(urlString)
		if err != nil {
			klog.Warningf("unable to parse url %q", urlString)
			return "", false
		}

		switch u.Path {
		case "/apis", "/api/v1", "/apis/v1":
			return u.Path, true
		default:
			tokens := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			if len(tokens) == 3 && tokens[0] == "apis" {
				return u.Path, true
			}
		}
		return "", false
	}

doAgain:
	changed := false
	for i := 0; i < len(l.Entries)-1; i++ {
		a := l.Entries[i]
		b := l.Entries[i+1]

		if a.Request.Method == "GET" && b.Request.Method == "GET" {
			aKey, aSwappable := isSwappable(a.Request.URL)
			bKey, bSwappable := isSwappable(b.Request.URL)
			if aSwappable && bSwappable {
				if aKey > bKey {
					l.Entries[i+1] = a
					l.Entries[i] = b
					changed = true
				}
			}
		}
	}
	if changed {
		goto doAgain
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
