package storage

// A Hook implements a lightweight watch on all objects, intended for use to mock controller behaviour.
type Hook interface {
	// OnWatchEvent is called whenever a watch event is created
	OnWatchEvent(ev *WatchEvent)
}
