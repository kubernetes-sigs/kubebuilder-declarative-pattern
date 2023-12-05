package controllers

// import (
// 	"fmt"
// 	"strconv"
// 	"strings"
// 	"sync"

// 	"k8s.io/apimachinery/pkg/runtime/schema"
// 	"k8s.io/apimachinery/pkg/types"
// 	"k8s.io/client-go/util/workqueue"
// 	"k8s.io/klog/v2"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"
// )

// type objectDependencyTracker struct {
// 	gvk     schema.GroupVersionKind
// 	subject types.NamespacedName
// 	q       workqueue.RateLimitingInterface

// 	dependencies dependencySet
// }

// func newObjectDependencyTracker
// type dependencySet map[schema.GroupKind]map[types.NamespacedName]int64

// var _ fmt.Stringer = dependencySet{}

// func (x dependencySet) String() string {
// 	var sb strings.Builder
// 	for gvk, objects := range x {
// 		if sb.Len() != 0 {
// 			fmt.Fprintf(&sb, ",")
// 		}
// 		fmt.Fprintf(&sb, "%v:[", gvk.String())
// 		for nn, rv := range objects {
// 			fmt.Fprintf(&sb, "%v@%d", nn.String(), rv)
// 		}
// 		fmt.Fprintf(&sb, "]")
// 	}
// 	return sb.String()
// }

// // var _ fmt.Formatter = dependencySet{}

// // func (x dependencySet) Format(f fmt.State, verb rune) {
// // 	var sb bytes.Buffer
// // 	for gvk, objects := range x {
// // 		if sb.Len() != 0 {
// // 			fmt.Fprintf(&sb, ",")
// // 		}
// // 		fmt.Fprintf(&sb, "%v:[", gvk.String())
// // 		for nn, rv := range objects {
// // 			fmt.Fprintf(&sb, "%v@%d", nn.String(), rv)
// // 		}
// // 		fmt.Fprintf(&sb, "]")
// // 	}
// // 	f.Write(sb.Bytes())
// // }
// type dependencySet map[schema.GroupKind]map[types.NamespacedName]int64

// var _ fmt.Stringer = dependencySet{}

// func (x dependencySet) String() string {
// 	var sb strings.Builder
// 	for gvk, objects := range x {
// 		if sb.Len() != 0 {
// 			fmt.Fprintf(&sb, ",")
// 		}
// 		fmt.Fprintf(&sb, "%v:[", gvk.String())
// 		for nn, rv := range objects {
// 			fmt.Fprintf(&sb, "%v@%d", nn.String(), rv)
// 		}
// 		fmt.Fprintf(&sb, "]")
// 	}
// 	return sb.String()
// }
// func (x dependencySet) Add(gk schema.GroupKind, nn types.NamespacedName, rv string) {
// 	rvInt, err := strconv.ParseInt(rv, 10, 64)
// 	if err != nil {
// 		klog.Fatalf("error parsing resource version %q", rv)
// 	}
// 	m := x[gk]
// 	if m == nil {
// 		m = make(map[types.NamespacedName]int64)
// 		x[gk] = m
// 	}
// 	m[nn] = rvInt
// }

// // type dependencyList struct {
// // 	mutex        sync.Mutex
// // 	 dependencies gknnIndex
// // }

// type invertedIndex struct {
// 	mutex    sync.Mutex
// 	trackers map[schema.GroupKind]map[types.NamespacedName][]*objectDependencyTracker
// }

// func (x *invertedIndex) onChange(gk schema.GroupKind, nn types.NamespacedName, rv int64) {
// 	x.mutex.Lock()
// 	defer x.mutex.Unlock()

// 	trackers := x.trackers[gk][nn]
// 	for _, tracker := range trackers {
// 		if tracker == nil {
// 			continue
// 		}
// 		seen, found := tracker.dependencies[gk][nn]
// 		if !found {
// 			panic("consistency error in invertedIndex: tracker entry not found")
// 		}
// 		if rv > seen {
// 			tracker.q.Add(reconcile.Request{NamespacedName: tracker.subject})
// 		}
// 	}
// }

// func (x *invertedIndex) update(tracker *objectDependencyTracker, oldDeps, newDeps map[schema.GroupKind]map[types.NamespacedName]int64) {
// 	x.mutex.Lock()
// 	defer x.mutex.Unlock()

// 	for gk, newIDs := range newDeps {
// 		if x.trackers[gk] == nil {
// 			x.trackers[gk] = make(map[types.NamespacedName][]*objectDependencyTracker)
// 		}

// 		oldIDs := oldDeps[gk]
// 		for newID := range newIDs {
// 			if _, found := oldIDs[newID]; !found {
// 				trackers := x.trackers[gk][newID]
// 				done := false
// 				for i := range trackers {
// 					if trackers[i] == nil {
// 						trackers[i] = tracker
// 						done = true
// 						break
// 					}
// 				}
// 				if !done {
// 					trackers = append(trackers, tracker)
// 				}
// 				x.trackers[gk][newID] = trackers
// 			}
// 		}
// 	}

// 	for gk, oldIDs := range oldDeps {
// 		if x.trackers[gk] == nil {
// 			x.trackers[gk] = make(map[types.NamespacedName][]*objectDependencyTracker)
// 		}

// 		newIDs := newDeps[gk]
// 		for oldID := range oldIDs {
// 			if _, found := newIDs[oldID]; !found {
// 				trackers := x.trackers[gk][oldID]
// 				done := false
// 				for i := range trackers {
// 					if trackers[i] == tracker {
// 						trackers[i] = nil
// 						if done {
// 							panic("consistency error in invertedIndex: double entry")
// 						}
// 						done = true
// 					}
// 				}
// 				if !done {
// 					panic("consistency error in invertedIndex: missing entry")
// 				}
// 				x.trackers[gk][oldID] = trackers
// 			}
// 		}
// 	}
// }

// // type objectKey struct {
// // 	gvk schema.GroupVersionKind

// // 	namespace string
// // 	name      string
// // }

// // type trackingClient struct {
// // 	inner client.Client
// // }

// // func (c *dependencyList) trackObject(gvk schema.GroupVersionKind, key types.NamespacedName) {
// // 	// scheme := c.Scheme()
// // 	// gvk, err := apiutil.GVKForObject(obj, scheme)
// // 	// if err != nil {
// // 	// 	return err
// // 	// }

// // 	c.mutex.Lock()
// // 	c.dependencies.Insert(gvk.Group, gvk.Kind, key.Namespace, key.Name)
// // 	c.mutex.Unlock()
// // }

// // type gknnIndex map[string]knnIndex

// // type knnIndex map[string]nnIndex

// // type nnIndex map[string]nIndex

// // type nIndex map[string]struct{}

// // func (x gknnIndex) Visit(visitor func(group, kind, namespace, name string)) {
// // 	for group, knn := range x {
// // 		for kind, nn := range knn {
// // 			for ns, names := range nn {
// // 				for name := range names {
// // 					visitor(group, kind, ns, name)
// // 				}
// // 			}
// // 		}
// // 	}
// // }
// // func (i gknnIndex) Insert(group, kind, namespace string, name string) {
// // 	v, found := i[group]
// // 	if !found {
// // 		v = make(knnIndex)
// // 		i[group] = v
// // 	}
// // 	v.insert(kind, namespace, name)
// // }

// // func (i knnIndex) insert(kind string, namespace string, name string) {
// // 	v, found := i[kind]
// // 	if !found {
// // 		v = make(nnIndex)
// // 		i[kind] = v
// // 	}
// // 	v.insert(namespace, name)
// // }

// // func (i nnIndex) insert(namespace string, name string) {
// // 	v, found := i[namespace]
// // 	if !found {
// // 		v = make(nIndex)
// // 		i[name] = v
// // 	}
// // 	v.insert(name)
// // }

// // func (i nIndex) insert(name string) {
// // 	i[name] = struct{}{}
// // }

// // // gvk, err := apiutil.GVKForObject(instance, r.scheme)
// // // if err != nil {
// // // 	return statusInfo, err
// // // }

// // // statusInfo.deps.trackObject(gvk, name)

// // // type statusInfo struct {
// // // 	declarative.StatusInfo
// // // 	deps dependencyList
// // // }
