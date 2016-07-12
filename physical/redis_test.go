package physical

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	//"github.com/garyburd/redigo/redis"
	//"golang.org/x/net/context"
)

func TestRedisBackend(t *testing.T) {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		t.SkipNow()
	}

	randPath := fmt.Sprintf("vault-leader-%d", time.Now().Unix())
	// TODO: Delete key when done.

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
}