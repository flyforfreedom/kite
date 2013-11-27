package main

import (
	"container/list"
	"koding/newkite/dnode"
	"koding/newkite/kite"
	"koding/newkite/protocol"
	"reflect"
	"sync"
)

type watcherHub struct {
	sync.RWMutex

	// Indexed by user to iterate faster when a notification comes.
	// Indexed by user because it is the first field in KontrolQuery.
	watchesByUser map[string]*list.List // List contains *watch

	// Indexed by Kite to remove them easily when Kite disconnects.
	watchesByKite map[*kite.RemoteKite][]*list.Element
}

type watch struct {
	query    *KontrolQuery
	callback dnode.Function
}

// const (
// 	  Register = iota
// 	Deregister
// )

func newWatcherHub() *watcherHub {
	return &watcherHub{
		watchesByUser: make(map[string]*list.List),
		watchesByKite: make(map[*kite.RemoteKite][]*list.Element),
	}
}

// RegisterWatcher saves the callbacks to invoke later
// when a Kite is registered/deregistered matching the query.
func (h *watcherHub) RegisterWatcher(r *kite.RemoteKite, q *KontrolQuery, callback dnode.Function) {
	h.Lock()
	defer h.Unlock()

	r.OnDisconnect(func() {
		h.Lock()
		defer h.Unlock()

		// Delete watch from watchesByUser
		for _, elem := range h.watchesByKite[r] {
			l := h.watchesByUser[q.Username]
			l.Remove(elem)

			if l.Len() == 0 {
				delete(h.watchesByUser, q.Username)
			}
		}

		delete(h.watchesByKite, r)
	})

	l, ok := h.watchesByUser[q.Username]
	if !ok {
		l = list.New()
		h.watchesByUser[q.Username] = l
	}

	elem := l.PushBack(&watch{q, callback})
	h.watchesByKite[r] = append(h.watchesByKite[r], elem)
}

// Notify is called when a Kite is registered by the user of this watcherHub.
// Calls the registered callbacks mathching to the kite.
func (h *watcherHub) Notify(kite *protocol.Kite, action protocol.KiteAction) {
	h.RLock()
	defer h.RUnlock()

	l, ok := h.watchesByUser[kite.Username]
	if ok {
		for e := l.Front(); e != nil; e = e.Next() {
			watch := e.Value.(*watch)
			if matches(kite, watch.query) {
				go watch.callback(&protocol.KiteEvent{action, *kite})
			}
		}
	}
}

// matches returns true if kite mathches to the query.
func matches(kite *protocol.Kite, query *KontrolQuery) bool {
	qv := reflect.ValueOf(*query)
	qt := qv.Type()

	for i := 0; i < qt.NumField(); i++ {
		qf := qv.Field(i)

		// Empty field in query matches everything.
		if qf.String() == "" {
			continue
		}

		// Compare field qf. query does not match if any field is different.
		kf := reflect.ValueOf(*kite).FieldByName(qt.Field(i).Name)
		if kf.String() != qf.String() {
			return false
		}
	}

	return true
}
