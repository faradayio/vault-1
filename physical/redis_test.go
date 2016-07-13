package physical

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
	//"github.com/garyburd/redigo/redis"
)

// To run this test, set up a local Redis instance and run:
//
//     docker run -p 6379:6379 -d redis
//     env REDIS_URL=redis://localhost:6379/9 make test TEST=./physical
func TestRedisBackend(t *testing.T) {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		t.SkipNow()
	}

	randPath := fmt.Sprintf("vault-leader-%d", time.Now().Unix())
	// TODO: Delete all matching keys when done.

	logger := log.New(os.Stderr, "", log.LstdFlags)

	b, err := NewBackend("redis", logger, map[string]string{
		"path": randPath,
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testBackend(t, b)
	testBackend_ListPrefix(t, b)

	ha, ok := b.(HABackend)
	if !ok {
		t.Fatalf("redis does not implement HABackend")
	}
	testHABackend(t, ha, ha)

	testRedisHAExpiration(t, ha)
}

// Since RedisLock provides a SimulateExpiration method, we can test what
// happens when the lock is lost unexpectedly.
func testRedisHAExpiration(t *testing.T, b HABackend) {
	// Get a lock.
	lock, err := b.LockWith("foo", "bar")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	redisLock, ok := lock.(*RedisLock)
	if !ok {
		t.Fatalf("redis did not return a RedisLock")
	}

	// Attempt to lock.
	leaderCh, err := redisLock.Lock(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leaderCh == nil {
		t.Fatalf("failed to get leader ch")
	}

	// Simulate a lock expiration manually.
	time.AfterFunc(50*time.Millisecond, func() {
		err := redisLock.simulateExpiration()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	select {
	case <-leaderCh:
		// Lock notified us that it failed, so we're good.
	case <-time.After(5 * time.Second):
		t.Fatalf("leaderCh did not report lock failure")
	}
}
