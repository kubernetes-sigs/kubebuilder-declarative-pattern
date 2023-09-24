package memorystorage

import (
	"context"
	"sync"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

func (resource *memoryResourceInfo) Watch(ctx context.Context, opt storage.WatchOptions, callback storage.WatchCallback) error {
	return resource.storage.watch(ctx, resource.gvk, opt, callback)
}

type resourceStorage struct {
	parent        *MemoryStorage
	GroupResource schema.GroupResource

	mutex   sync.Mutex
	watches []*watch

	resourceVersionClock *resourceVersionClock

	objects map[types.NamespacedName]*unstructured.Unstructured
}

type resourceVersionClock struct {
	now atomic.Int64
}

func (c *resourceVersionClock) Now() int64 {
	return c.now.Load()
}

func (c *resourceVersionClock) GetNext() int64 {
	return c.now.Add(1)
}

type watch struct {
	callback storage.WatchCallback
	opt      storage.WatchOptions
	errChan  chan error
}

func (r *resourceStorage) watch(ctx context.Context, gvk schema.GroupVersionKind, opt storage.WatchOptions, callback storage.WatchCallback) error {
	w := &watch{
		callback: callback,
		opt:      opt,
		errChan:  make(chan error),
	}

	r.mutex.Lock()
	pos := -1
	for i := range r.watches {
		if r.watches[i] == nil {
			r.watches[i] = w
			pos = i
			break
		}
	}
	if pos == -1 {
		r.watches = append(r.watches, w)
		pos = len(r.watches) - 1
	}
	r.mutex.Unlock()

	// TODO: Delay / buffer watch notifications until after the list

	// TODO: Only send list if no rv specified?

	r.mutex.Lock()
	for _, obj := range r.objects {
		if opt.Namespace != "" {
			if obj.GetNamespace() != opt.Namespace {
				continue
			}
		}

		ev := storage.BuildWatchEvent(gvk, "ADDED", obj)
		if err := w.callback(ev); err != nil {
			klog.Warningf("error sending backfill watch notification; stopping watch: %v", err)

			// remove watch from list
			r.watches[pos] = nil

			return err
		}
	}
	r.mutex.Unlock()

	return <-w.errChan
}

func (r *resourceStorage) broadcastEventHoldingLock(ctx context.Context, gvk schema.GroupVersionKind, evType string, u *unstructured.Unstructured) {
	ev := storage.BuildWatchEvent(gvk, evType, u)

	r.parent.fireOnWatchEvent(ev)

	// r.mutex should be locked
	for i := range r.watches {
		w := r.watches[i]
		if w == nil {
			continue
		}
		if w.opt.Namespace != "" && ev.Namespace != w.opt.Namespace {
			continue
		}
		if err := w.callback(ev); err != nil {
			klog.Warningf("error sending watch notification; stopping watch: %v", err)
			w.errChan <- err
			r.watches[i] = nil
		}
	}
}
