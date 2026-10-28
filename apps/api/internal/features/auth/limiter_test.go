package auth

import (
	"testing"
	"time"
)

func TestFailureLimiter(t *testing.T) {
	l := newFailureLimiter(3, 15*time.Minute)
	now := time.Now()

	for range 3 {
		if l.blocked("a@x.test", now) {
			t.Fatal("blocked too early")
		}
		l.fail("a@x.test", now)
	}
	if !l.blocked("a@x.test", now) {
		t.Error("should be blocked after 3 failures")
	}
	if l.blocked("b@x.test", now) {
		t.Error("other keys must not be affected")
	}
	if l.blocked("a@x.test", now.Add(16*time.Minute)) {
		t.Error("should unblock after the window")
	}

	l.fail("c@x.test", now)
	l.reset("c@x.test")
	if len(l.fails["c@x.test"]) != 0 {
		t.Error("reset should clear failures")
	}
}
