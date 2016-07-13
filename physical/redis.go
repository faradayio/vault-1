package physical

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

// EtcdBackend is a physical backend that stores data at specific
// prefix within Etcd. It is used for most production situations as
// it allows Vault to run on multiple machines in a highly-available manner.
type RedisBackend struct {
	path   string
	conn   *redis.Conn
	logger *log.Logger
}

func newRedisBackend(conf map[string]string, logger *log.Logger) (Backend, error) {
	// Get the Redis path from the configuration.
	path, ok := conf["path"]
	if !ok {
		path = "vault"
	}

	// Get the Redis URL from our environment.
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://127.0.0.1:6379"
	}
	addr := strings.TrimPrefix(url, "redis://")

	// Connect to Redis.
	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &RedisBackend{
		path:   path,
		conn:   &conn,
		logger: logger,
	}, nil
}

func (c *RedisBackend) Put(entry *Entry) error {
	encoded := base64.StdEncoding.EncodeToString(entry.Value)
	realKey := fmt.Sprintf("%s/%s", c.path, entry.Key)
	_, err := (*c.conn).Do("SET", realKey, encoded)
	return err
}

func (c *RedisBackend) Get(key string) (*Entry, error) {
	reply, err := (*c.conn).Do("GET", fmt.Sprintf("%s/%s", c.path, key))
	if err != nil {
		return nil, err
	}
	if reply == nil {
		// No value at this key, so don't try to decode.
		return nil, nil
	}

	replyStr, err := redis.String(reply, err)
	if err != nil {
		return nil, err
	}

	value, err := base64.StdEncoding.DecodeString(replyStr)
	if err != nil {
		return nil, err
	}

	return &Entry{
		Key:   key,
		Value: value,
	}, nil
}

func (c *RedisBackend) Delete(key string) error {
	_, err := (*c.conn).Do("DEL", fmt.Sprintf("%s/%s", c.path, key))
	return err
}

func (c *RedisBackend) List(prefix string) ([]string, error) {
	// Construct a "directory"-style name with a trailing slash from
	// `prefix`.  But`prefix` may be the empty string, so be careful
	// how we add "/".
	realPrefix := fmt.Sprintf("%s/%s", c.path, prefix)
	if !strings.HasSuffix(realPrefix, "/") {
		realPrefix += "/"
	}

	// Ask Redis to list all keys beneath our prefix.  Note that this
	// is not terribly efficient if you have a lot of keys, but we hope
	// it's not a common operating.  There are more complex APIs for
	// doing this incrementally.
	reply, err := (*c.conn).Do("KEYS", realPrefix+"*")
	matches, err := redis.Strings(reply, err)

	// TODO - Don't recurse subdirectories.  This means stripping
	// everything after "/" and removing duplicates.
	results := make([]string, 0)
	dirs := make(map[string]bool)
	for _, match := range matches {
		// Remove the path that we were querying from the results.
		match = strings.TrimPrefix(match, realPrefix)

		slashPos := strings.Index(match, "/")
		if slashPos == -1 {
			// We have an ordinary file, so return it directly.
			results = append(results, match)
		} else {
			// Keep the trailing slash.
			dir := match[0 : slashPos+1]

			// Have we seen this directory already?  If not,
			// return it and mark it as seen.
			if _, present := dirs[dir]; !present {
				results = append(results, dir)
				dirs[dir] = true
			}
		}
	}

	return results, err
}

func (c *RedisBackend) LockWith(key, value string) (Lock, error) {
	return &RedisLock{
		key:   fmt.Sprintf("%s/_lock/%s", c.path, key),
		value: value,
		conn:  c.conn,
	}, nil
}

type RedisLock struct {
	key   string
	value string
	conn  *redis.Conn
}

// Acquire the lock.  To interrupt lock acquistion, close stopCh.  The
// returned channel will be closed if the lock is lost.
func (c *RedisLock) Lock(stopCh <-chan struct{}) (<-chan struct{}, error) {
	// Loop until we get a non-nil reply from Redis indicating a
	// successful operation.
	var reply interface{}
	for reply == nil {
		// Attempt to set our key.  "NX" means to only set the key
		// if it does not exist, and "PX" specifies a timeout in
		// milliseconds.
		reply, err := (*c.conn).Do("SET", c.key, c.value, "NX", "PX", "30000")
		if err != nil {
			// We got an error communicating with Redis, so
			// fail outright.
			return nil, err
		}
		if reply != nil {
			// We got back a non-nil response, which means we
			// set the key and can stop looping.
			break
		}

		// Create a timeout channel that will send a message after
		// 5 seconds.
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(5 * time.Second)
			timeout <- true
		}()

		select {
		case <-stopCh:
			// Lock acquisition was cancelled by our caller.
			return nil, nil
		case <-timeout:
			// Timeout fired, so here we go again.
		}
	}

	return make(chan struct{}), nil
}

// Release the lock.
func (c *RedisLock) Unlock() error {
	unlockScript := redis.NewScript(1, `
if redis.call("get",KEYS[1]) == ARGV[1] then
    return redis.call("del",KEYS[1])
else
    return 0
end
`)
	reply, err := unlockScript.Do(*c.conn, c.key, c.value)
	deleted, err := redis.Int(reply, err)
	if err != nil {
		return err
	}
	if deleted == 0 {
		// I presume we ought to return an error here.
		return fmt.Errorf("redis: tried to unlock somebody else's lock")
	}
	return nil
}

// Returns the value of the lock and if it is held.
func (c *RedisLock) Value() (bool, string, error) {
	reply, err := (*c.conn).Do("GET", c.key)
	if err != nil {
		// Could not read value.
		return false, "", err
	}
	if reply == nil {
		// Nobody is holding the lock.
		return false, "", nil
	}

	// Somebody is holding the lock, so report the value.
	value, err := redis.String(reply, err)
	return true, value, err
}

//type Lock interface {
//	// Lock is used to acquire the given lock
//	// The stopCh is optional and if closed should interrupt the lock
//	// acquisition attempt. The return struct should be closed when
//	// leadership is lost.
//	Lock(stopCh <-chan struct{}) (<-chan struct{}, error)
//
//	// Unlock is used to release the lock
//	Unlock() error
//
//	// Returns the value of the lock and if it is held
//	Value() (bool, string, error)
//}
