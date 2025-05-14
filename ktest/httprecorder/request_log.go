package httprecorder

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
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

func (e *LogEntry) FormatHTTP(pretty bool) string {
	var b strings.Builder
	b.WriteString(e.Request.FormatHTTP(pretty))
	b.WriteString("\n")
	b.WriteString(e.Response.FormatHTTP(pretty))
	return b.String()
}

func (r *Request) FormatHTTP(pretty bool) string {
	var w strings.Builder
	w.WriteString(fmt.Sprintf("%s %s\n", r.Method, r.URL))
	var keys []string
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range r.Header[k] {
			w.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	w.WriteString("\n")
	writeBody(&w, r.Body, pretty)
	return w.String()
}

func writeBody(w io.StringWriter, body string, pretty bool) {
	if body == "" {
		return
	}

	if pretty {
		var obj any
		if err := json.Unmarshal([]byte(body), &obj); err == nil {
			b, err := json.MarshalIndent(obj, "", "  ")
			if err == nil {
				body = string(b)
			}
		}
	}

	w.WriteString(body)
	w.WriteString("\n")
}

func (l *RequestLog) ReplaceTimestamp() {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for _, entry := range l.entries {
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
		// Use a fixed timestamp for golden tests.
		t := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
		cond["lastTransitionTime"] = t.Format("2006-01-02T15:04:05Z07:00")
	}
	b, _ := json.Marshal(u)
	return string(b)
}

func (r *Response) FormatHTTP(pretty bool) string {
	// Skip empty responses (e.g. from streaming watch)
	if r.Status == "" && r.StatusCode == 0 && r.Body == "" {
		return ""
	}

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
	writeBody(&b, r.Body, pretty)
	return b.String()
}

type RequestLog struct {
	// We have seen dropped logs from concurrent requests
	mutex sync.Mutex

	entries []*LogEntry
}

func (l *RequestLog) AddEntry(entry *LogEntry) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.entries = append(l.entries, entry)
}

func (l *RequestLog) FormatHTTP(pretty bool) string {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	var actual []string
	for _, entry := range l.entries {
		s := entry.FormatHTTP(pretty)
		actual = append(actual, s)
	}
	return strings.Join(actual, "\n---\n\n")
}

func (l *RequestLog) ReplaceURLPrefix(old, new string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i := range l.entries {
		r := &l.entries[i].Request
		if strings.HasPrefix(r.URL, old) {
			r.URL = new + strings.TrimPrefix(r.URL, old)
		}
	}
}

func (l *RequestLog) RemoveHeader(k string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i := range l.entries {
		request := &l.entries[i].Request
		request.Header.Del(k)

		response := &l.entries[i].Response
		response.Header.Del(k)
	}
}

func (l *RequestLog) ReplaceHeader(k string, v string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i := range l.entries {
		request := &l.entries[i].Request
		if len(request.Header.Values(k)) != 0 {
			request.Header.Set(k, v)
		}

		response := &l.entries[i].Response
		if len(response.Header.Values(k)) != 0 {
			response.Header.Set(k, v)
		}
	}
}

// SortGETs attempts to normalize parallel requests.
// Consecutive GET requests are sorted alphabetically.
func (l *RequestLog) SortGETs() {
	l.mutex.Lock()
	defer l.mutex.Unlock()

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
	for i := 0; i < len(l.entries)-1; i++ {
		a := l.entries[i]
		b := l.entries[i+1]

		if a.Request.Method == "GET" && b.Request.Method == "GET" {
			aKey, aSwappable := isSwappable(a.Request.URL)
			bKey, bSwappable := isSwappable(b.Request.URL)
			if aSwappable && bSwappable {
				if aKey > bKey {
					l.entries[i+1] = a
					l.entries[i] = b
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

func (l *RequestLog) RegexReplaceURL(t *testing.T, find string, replace string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	r, err := regexp.Compile(find)
	if err != nil {
		t.Fatalf("failed to compile regex %q: %v", find, err)
	}

	for i := range l.entries {
		request := &l.entries[i].Request
		u := request.URL

		u = r.ReplaceAllString(u, replace)
		request.URL = u
	}
}

// RewriteBodies rewrites the bodies of the requests and responses.
// The function fn is called with the body as a map[string]any.
func (l *RequestLog) RewriteEntries(t *testing.T, fn func(entry *LogEntry)) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i := range l.entries {
		entry := l.entries[i]
		fn(entry)
	}
}

// RewriteBodies rewrites the bodies of the requests and responses.
// The function fn is called with the body as a map[string]any.
func (l *RequestLog) RewriteBodies(t *testing.T, fn func(body map[string]any)) {
	l.RewriteEntries(t, func(entry *LogEntry) {
		entry.Request.Body = rewriteBody(t, entry.Request.Body, fn)
		entry.Response.Body = rewriteBody(t, entry.Response.Body, fn)
	})
}

// rewriteBody rewrites the body of a request or response, assuming it is a JSON object.
func rewriteBody(t *testing.T, body string, fn func(body map[string]any)) string {
	if body == "" {
		return body
	}

	// Ignore values we replaced
	if strings.HasPrefix(body, "// ") {
		return body
	}

	var obj any
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		t.Errorf("failed to unmarshal body: %v", err)
		return body
	}

	m, ok := obj.(map[string]any)
	if !ok {
		return body
	}
	fn(m)

	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Errorf("failed to marshal body: %v", err)
		return body
	}
	return string(b)
}
