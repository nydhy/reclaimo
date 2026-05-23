package events

import "sync"

type Store interface {
	Append(event Event)
	List() []Event
	Subscribe() (<-chan Event, func())
}

type MemoryStore struct {
	mu          sync.RWMutex
	events      []Event
	subscribers map[chan Event]struct{}
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{subscribers: make(map[chan Event]struct{})}
}

func (s *MemoryStore) Append(event Event) {
	s.mu.Lock()
	s.events = append(s.events, event)
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *MemoryStore) List() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

func (s *MemoryStore) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)

	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		close(ch)
		s.mu.Unlock()
	}

	return ch, cancel
}
