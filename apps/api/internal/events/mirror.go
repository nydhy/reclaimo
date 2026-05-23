package events

import "log"

type Sink interface {
	Write(event Event) error
}

type MirrorStore struct {
	primary *MemoryStore
	sinks   []Sink
}

func NewMirrorStore(primary *MemoryStore, sinks ...Sink) *MirrorStore {
	return &MirrorStore{primary: primary, sinks: sinks}
}

func (s *MirrorStore) Append(event Event) {
	s.primary.Append(event)

	for _, sink := range s.sinks {
		if err := sink.Write(event); err != nil {
			log.Printf("event sink write failed: type=%s err=%v", event.Type, err)
		}
	}
}

func (s *MirrorStore) List() []Event {
	return s.primary.List()
}

func (s *MirrorStore) Subscribe() (<-chan Event, func()) {
	return s.primary.Subscribe()
}
