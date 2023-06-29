package watchset

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Manager struct {
	client     dynamic.Interface
	restMapper meta.RESTMapper

	mutex          sync.Mutex
	interests      []*InterestSet
	interestsByGVR map[schema.GroupVersionResource][]*InterestSet
	watchesByGVR   map[schema.GroupVersionResource]*gvrWatcher
}

func NewManager(mgr manager.Manager) (*Manager, error) {
	dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("error building dynamic client: %w", err)
	}
	m := &Manager{
		client:         dynamicClient,
		restMapper:     mgr.GetRESTMapper(),
		interestsByGVR: make(map[schema.GroupVersionResource][]*InterestSet),
		watchesByGVR:   make(map[schema.GroupVersionResource]*gvrWatcher),
	}
	return m, nil
}

func (w *Manager) newInterestSet(callback func()) *InterestSet {
	interest := &InterestSet{
		parent:   w,
		byGVR:    make(map[schema.GroupVersionResource]*interestSetGVR),
		callback: callback,
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.interests = append(w.interests, interest)

	return interest
}

func (w *Manager) updateInterests() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	watchsetsByGVR := make(map[schema.GroupVersionResource][]*InterestSet)

	for _, interest := range w.interests {
		if interest.closed {
			// TODO: Clean up closed InterestSets from the slice
			continue
		}
		for gvr := range interest.byGVR {
			watchsetsByGVR[gvr] = append(watchsetsByGVR[gvr], interest)
		}
	}
	w.interestsByGVR = watchsetsByGVR

	// Make sure we are running watches for all interests
	for gvr := range watchsetsByGVR {
		watcher := w.watchesByGVR[gvr]
		if watcher == nil {
			klog.Infof("starting new watch for gvr %v", gvr)
			watcher = &gvrWatcher{
				gvr:        gvr,
				parent:     w,
				restMapper: w.restMapper,
				client:     w.client,
			}

			ctx, cancel := context.WithCancel(context.Background())
			watcher.cancel = cancel

			go watcher.watchForever(ctx)

			w.watchesByGVR[gvr] = watcher
		}
	}

	// Close down any watches no longer of interest
	for gvr, watcher := range w.watchesByGVR {
		if _, found := watchsetsByGVR[gvr]; found {
			continue
		}

		klog.Infof("stopping watch for gvr %v", gvr)
		watcher.cancel()
		delete(w.watchesByGVR, gvr)
	}
}

func (w *Manager) onEvent(gvr schema.GroupVersionResource, ev *objectEvent) {
	// TODO: rw-mutex
	// TODO: register interests directly with watchers?
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, interest := range w.interestsByGVR[gvr] {
		interest.onEvent(gvr, ev)
	}
}

type gvrWatcher struct {
	client     dynamic.Interface
	restMapper meta.RESTMapper

	// TODO: Should we replace this with a direct pointer to the target(s)?
	parent *Manager

	// gvk schema.GroupVersionKind
	gvr schema.GroupVersionResource

	cancel func()
}

func (w *gvrWatcher) watchForever(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := w.watchOnce(ctx); err != nil {
			klog.Warningf("error from watch; will reconect: %v", err)
		} else {
			klog.Warningf("watch closed; will reconect")
		}
		time.Sleep(5 * time.Second)
	}
}

func (w *gvrWatcher) watchOnce(ctx context.Context) error {
	// if w.gvr.Empty() {
	// 	restMapping, err := w.restMapper.RESTMapping(w.gvk.GroupKind(), w.gvk.Version)
	// 	if err != nil {
	// 		return fmt.Errorf("unable to get rest mapping for %v: %w", w.gvk, err)
	// 	}
	// 	w.gvr = restMapping.Resource
	// }

	listOptions := metav1.ListOptions{
		Watch:               true,
		AllowWatchBookmarks: true,
		// TODO: ResourceVersion (maybe)
	}
	// TODO: Can we use metadata only (probably not if we have to compute the health)
	watcher, err := w.client.Resource(w.gvr).Watch(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}

	// Always clean up
	defer watcher.Stop()

	for ev := range watcher.ResultChan() {
		switch ev.Type {
		case watch.Bookmark:
			// TODO: Update resource version?

		case watch.Error:
			klog.Warningf("got error on watch stream for %v: %v", w.gvr, ev)
			return fmt.Errorf("got error on watch stream")

		case watch.Added, watch.Modified, watch.Deleted:
			objectEvent, err := buildObjectEvent(&ev)
			if err != nil {
				return fmt.Errorf("error building object event: %w", err)
			}
			w.parent.onEvent(w.gvr, objectEvent)

		default:
			klog.Warningf("got unknown message on watch stream: %v", ev)
			return fmt.Errorf("got unknown message watch stream")
		}
	}

	return nil
}

type objectEvent struct {
	EventType watch.EventType
	Labels    map[string]string
	ID        types.NamespacedName
}

func buildObjectEvent(ev *watch.Event) (*objectEvent, error) {
	o := &objectEvent{
		EventType: ev.Type,
	}
	if ev.Object == nil {
		return nil, fmt.Errorf("object not set in event")
	}
	accessor, err := meta.Accessor(ev.Object)
	if err != nil {
		klog.Fatalf("failed to get accessor for %T: %v", ev.Object, err)
	}

	o.ID = types.NamespacedName{
		Namespace: accessor.GetNamespace(),
		Name:      accessor.GetName(),
	}
	o.Labels = accessor.GetLabels()

	return o, nil
}
