package auth

import (
	"sync"
	"time"
)

// failureLimiter blocks a key (an email) after too many failures in a window.
// In memory, so it limits per instance; moves to platform/cache with several instances.
type failureLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
}

func newFailureLimiter(maxFailures int, window time.Duration) *failureLimiter {
	return &failureLimiter{max: maxFailures, window: window, fails: map[string][]time.Time{}}
}

func (l *failureLimiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key, now)) >= l.max
}

func (l *failureLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key] = append(l.recent(key, now), now)
	if len(l.fails) > 10_000 { // bound memory: drop keys with no recent failures
		for k := range l.fails {
			if len(l.recent(k, now)) == 0 {
				delete(l.fails, k)
			}
		}
	}
}

func (l *failureLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// recent returns failures inside the window. Callers hold l.mu.
func (l *failureLimiter) recent(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	keep := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if len(keep) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = keep
	return keep
}
