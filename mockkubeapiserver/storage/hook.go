package storage

// A Hook implements a lightweight watch on all objects, intended for use to mock controller behaviour.
type Hook interface {
	OnWatchEvent(ev *WatchEvent)
}
