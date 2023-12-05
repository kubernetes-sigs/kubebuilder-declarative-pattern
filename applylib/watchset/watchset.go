package watchset

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// func newObjectDependencyTracker
type dependencySet map[schema.GroupKind]map[types.NamespacedName]int64

var _ fmt.Stringer = dependencySet{}

func (x dependencySet) String() string {
	var sb strings.Builder
	for gvk, objects := range x {
		if sb.Len() != 0 {
			fmt.Fprintf(&sb, ",")
		}
		fmt.Fprintf(&sb, "%v:[", gvk.String())
		for nn, rv := range objects {
			fmt.Fprintf(&sb, "%v@%d", nn.String(), rv)
		}
		fmt.Fprintf(&sb, "]")
	}
	return sb.String()
}

func (x dependencySet) Add(gk schema.GroupKind, nn types.NamespacedName, rv string) {
	rvInt, err := strconv.ParseInt(rv, 10, 64)
	if err != nil {
		klog.Fatalf("error parsing resource version %q", rv)
	}
	m := x[gk]
	if m == nil {
		m = make(map[types.NamespacedName]int64)
		x[gk] = m
	}
	m[nn] = rvInt
}
