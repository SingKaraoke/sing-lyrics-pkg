package auth

import (
	"sync"
	"time"
)

// NonceStore is an in-process replay-protection cache: fine for a single
// instance (see docs/ARCHITECTURE.md §1a) - swap for Redis if you ever run
// more than one upload-api instance behind a load balancer.
type NonceStore struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
}

func NewNonceStore(ttl time.Duration) *NonceStore {
	s := &NonceStore{seen: make(map[string]time.Time), ttl: ttl}
	go s.sweepLoop()
	return s
}

// CheckAndRemember returns false if the (device, nonce) pair was already seen
// within the TTL window, true (and records it) otherwise.
func (s *NonceStore) CheckAndRemember(deviceID, nonce string) bool {
	key := deviceID + ":" + nonce
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if seenAt, ok := s.seen[key]; ok && now.Sub(seenAt) < s.ttl {
		return false
	}
	s.seen[key] = now
	return true
}

func (s *NonceStore) sweepLoop() {
	ticker := time.NewTicker(s.ttl)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-s.ttl)
		s.mu.Lock()
		for k, t := range s.seen {
			if t.Before(cutoff) {
				delete(s.seen, k)
			}
		}
		s.mu.Unlock()
	}
}
