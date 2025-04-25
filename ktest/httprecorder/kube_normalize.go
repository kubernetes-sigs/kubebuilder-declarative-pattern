package httprecorder

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

// placeholderTime is a placeholder time that is used to replace the timestamps, so that we can golden test.
var placeholderTime = metav1.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

// NormalizeKubeRequestLog normalizes a kube request log for golden testing,
// it replaces ephemeral values with placeholder values,
// sorts requests into a predictable order,
// and removes non-deterministic headers.
func NormalizeKubeRequestLog(t *testing.T, requestLog *RequestLog, restConfig *rest.Config) {
	requestLog.RewriteEntries(t, func(entry *LogEntry) {
		u, err := url.Parse(entry.Request.URL)
		if err != nil {
			t.Errorf("error parsing url %q: %v", entry.Request.URL, err)
			return
		}
		u.Host = "kube-apiserver"
		u.Scheme = "https"
		entry.Request.URL = u.String()
	})

	// Replace URLs that include a hash value - this is used by API discovery, but introduces effectively random values into our golden tests
	requestLog.RewriteEntries(t, func(entry *LogEntry) {
		u, err := url.Parse(entry.Request.URL)
		if err != nil {
			t.Errorf("error parsing url %q: %v", entry.Request.URL, err)
			return
		}
		q := u.Query()
		if q.Get("hash") != "" {
			q.Set("hash", "some-hash")
			u.RawQuery = q.Encode()
			entry.Request.URL = u.String()
		}
	})

	// Remove very-long well-known response bodies (e.g. API discovery)
	requestLog.RewriteEntries(t, func(entry *LogEntry) {
		replaceResponseBody := ""
		s := entry.Request.URL
		// Remove querystring
		s, _, _ = strings.Cut(s, "?")
		switch s {
		case "https://kube-apiserver/openapi/v3",
			"https://kube-apiserver/openapi/v3/api/v1",
			"https://kube-apiserver/api/v1",
			"https://kube-apiserver/api",
			"https://kube-apiserver/apis":
			// Remove massive API discovery documents
			replaceResponseBody = "// discovery response removed for length"
		}
		if replaceResponseBody != "" {
			entry.Response.Body = replaceResponseBody
		}
	})

	requestLog.RemoveUserAgent()

	requestLog.RemoveHeader("X-Kubernetes-Pf-Flowschema-Uid")
	requestLog.RemoveHeader("X-Kubernetes-Pf-Prioritylevel-Uid")
	requestLog.RemoveHeader("Audit-Id")
	requestLog.RemoveHeader("Kubectl-Session")

	requestLog.RemoveHeader("Content-Length")
	requestLog.RemoveHeader("Date")

	requestLog.ReplaceHeader("ETag", "(removed)")
	requestLog.ReplaceHeader("Expires", "(removed)")
	requestLog.ReplaceHeader("Last-Modified", "(removed)")

	requestLog.RewriteBodies(t, func(body map[string]any) {
		u := unstructured.Unstructured{Object: body}
		uid := u.GetUID()
		if uid != "" {
			u.SetUID("fake-uid")
		}

		replaceIfPresent(t, body, placeholderTime.Format(time.RFC3339), "metadata", "creationTimestamp")

		if managedFields := u.GetManagedFields(); managedFields != nil {
			for i := range managedFields {
				managedFields[i].Time = &placeholderTime
			}
			u.SetManagedFields(managedFields)
		}
	})

	// Rewrite the resource version to a predictable value
	// We don't want to use a fixed value, because we want to verify our behaviour here.
	{
		resourceVersions := sets.New[int]()
		requestLog.RewriteBodies(t, func(body map[string]any) {
			u := unstructured.Unstructured{Object: body}
			rv := u.GetResourceVersion()
			if rv != "" {
				// These aren't guaranteed to be ints, but in practice they are, and this is test code.
				rvInt, err := strconv.Atoi(rv)
				if err != nil {
					t.Errorf("error converting resource version %q to int: %v", rv, err)
					return
				}
				resourceVersions.Insert(rvInt)
			}
		})
		resourceVersionsList := resourceVersions.UnsortedList()
		sort.Ints(resourceVersionsList)
		rewriteResourceVersion := map[string]string{}
		for i, rv := range resourceVersionsList {
			rewriteResourceVersion[strconv.Itoa(rv)] = strconv.Itoa(1000 + i)
		}
		requestLog.RewriteBodies(t, func(body map[string]any) {
			u := unstructured.Unstructured{Object: body}
			rv := u.GetResourceVersion()
			u.SetResourceVersion(rewriteResourceVersion[rv])
		})
	}

	requestLog.SortGETs()
}

func replaceIfPresent(t *testing.T, body map[string]any, replace string, path ...string) {
	u := unstructured.Unstructured{Object: body}
	_, found, err := unstructured.NestedFieldNoCopy(u.Object, path...)
	if err != nil {
		t.Errorf("error getting nested field %v: %v", path, err)
		return
	}
	if found {
		unstructured.SetNestedField(u.Object, replace, path...)
	}
}
