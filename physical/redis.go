package physical

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

// RedisBackend is a physical backend that stores data at a specific prefix
// within Redis.  This is not especially useful as an ordinary
// secret-storage backend, unless you set up Redis persistence.
//
// But this backend offers a relatively simple way to set up a "medium
// availability" Vault cluster, where you use S3 or another backend for
// storage, and a single Redis server as a "ha_backend" to handle leader
// elections.  If the Redis server fails, then all vault servers should go
// to standby until Redis returns.
//
// This is potentially useful because Redis servers are ubiquitous and
// surprisingly reliable, and because not everybody wants to set up and
// administer either Consul or etcd for a single leader lock.  Also,
// high-quality hosted and managed Redis services are widely available.
type RedisBackend struct {
	path     string
	pool     *redis.Pool
	logger   *log.Logger
	lockConf *RedisLockConfig
}

// Configuration options for a RedisLock.
type RedisLockConfig struct {
	// How long should the leader TTL last?
	leaderTTL               time.Duration
	// How long should the leader wait before renewing the TTL?  This
	// should be no more than half of leaderTTL.
	leaderTTLRenewInterval  time.Duration
	// How often should a standby server attempt to take the lock?
	leaderLockRetryInterval time.Duration
}

func parseRedisTimeout(conf map[string]string, key string, defaultVal time.Duration) (time.Duration, error) {
	timeoutStr, ok := conf[key]
	if !ok {
		return defaultVal, nil
	}
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return 0, err
	}
	return time.Duration(timeout) * time.Millisecond, nil
}

func newRedisBackend(conf map[string]string, logger *log.Logger) (Backend, error) {
	// Get the Redis path from the configuration.
	path, ok := conf["path"]
	if !ok {
		path = "vault"
	}

	// Get the Redis URL from either our config file or our environment.
	url, ok := conf["url"]
	if !ok {
		url = "redis://127.0.0.1:6379"
	}
	urlEnv := os.Getenv("REDIS_URL")
	if urlEnv != "" {
		url = urlEnv
	}

	// Get our lock timeouts.
	leaderTTL, err := parseRedisTimeout(conf, "leader_ttl", 30*time.Second)
	if err != nil {
		return nil, err
	}
	leaderTTLRenewInterval, err := parseRedisTimeout(conf, "leader_ttl_renew_interval", 1*time.Second)
	if err != nil {
		return nil, err
	}
	leaderLockRetryInterval, err := parseRedisTimeout(conf, "leader_lock_retry_interval", 5*time.Second)
	if err != nil {
		return nil, err
	}
	if 2*leaderTTLRenewInterval > leaderTTL {
		return nil, fmt.Errorf("leader_ttl_renew_interval must be no more than half of leader_ttl")
	}

	// Create a Redis connection pool so that we can access Redis from
	// multiple goroutines in parallel.  This was adapted from the
	// Redigo docs.
	pool := &redis.Pool{
		MaxIdle:     2,
		IdleTimeout: 300 * time.Second,
		Dial: func() (redis.Conn, error) {
			// Use DialURL to handle database numbers,
			// authentication, etc.
			conn, err := redis.DialURL(url)
			return conn, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	return &RedisBackend{
		path:     path,
		pool:     pool,
		logger:   logger,
		lockConf: &RedisLockConfig{
			leaderTTL: leaderTTL,
			leaderTTLRenewInterval: leaderTTLRenewInterval,
			leaderLockRetryInterval: leaderLockRetryInterval,
		},
	}, nil
}

func (c *RedisBackend) keyPath(key string) string {
	return fmt.Sprintf("%s/data/%s", c.path, key)
}

func (c *RedisBackend) Put(entry *Entry) error {
	conn := c.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", c.keyPath(entry.Key), entry.Value)
	return err
}

func (c *RedisBackend) Get(key string) (*Entry, error) {
	conn := c.pool.Get()
	defer conn.Close()

	reply, err := conn.Do("GET", c.keyPath(key))
	if err != nil {
		return nil, err
	}
	if reply == nil {
		// No value at this key, so don't try to decode.
		return nil, nil
	}

	value, err := redis.String(reply, err)
	if err != nil {
		return nil, err
	}

	return &Entry{
		Key:   key,
		Value: []byte(value),
	}, nil
}

func (c *RedisBackend) Delete(key string) error {
	conn := c.pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", c.keyPath(key))
	return err
}

func (c *RedisBackend) List(prefix string) ([]string, error) {
	conn := c.pool.Get()
	defer conn.Close()

	// Construct a "directory"-style name with a trailing slash from
	// `prefix`.  But`prefix` may be the empty string, so be careful
	// how we add "/".
	realPrefix := c.keyPath(prefix)
	if !strings.HasSuffix(realPrefix, "/") {
		realPrefix += "/"
	}

	// Ask Redis to list all keys beneath our prefix.  Note that this
	// is not terribly efficient if you have a lot of keys, but we hope
	// it's not a common operating.  There are more complex APIs for
	// doing this incrementally.
	reply, err := conn.Do("KEYS", realPrefix+"*")
	matches, err := redis.Strings(reply, err)

	// The KEYS command recurses "subdirectories", but we don't want to
	// return the recursive values in our output.  This means stripping
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
		key:      fmt.Sprintf("%s/_lock/%s", c.path, key),
		value:    value,
		pool:     c.pool,
		logger:   c.logger,
		lockConf: c.lockConf,
	}, nil
}

// A lock implemented using Redis.  This is based on the single-Redis
// example code used to explain Redlock at http://redis.io/topics/distlock
// but it might be useful to upgrade this to a full Redlock implementation.
type RedisLock struct {
	key      string
	value    string
	pool     *redis.Pool
	logger   *log.Logger
	lockConf *RedisLockConfig
}

// Acquire the lock.  To interrupt lock acquistion, close stopCh.  The
// returned channel will be closed if the lock is lost.
func (c *RedisLock) Lock(stopCh <-chan struct{}) (<-chan struct{}, error) {
	// Round up leaderTTL to the nearest second and convert to a
	// string.  We do this because EXPIRE only has 1-second accuracy.
	ttlSeconds := float64(c.lockConf.leaderTTL) / float64(time.Second);
	ttlStr := strconv.Itoa(int(math.Ceil(ttlSeconds)));

	// Loop until we get a non-nil reply from Redis indicating a
	// successful operation.
	var reply interface{}
	for reply == nil {
		conn := c.pool.Get()
		defer conn.Close()

		// Attempt to set our key.  "NX" means to only set the key
		// if it does not exist, and "EX" specifies a timeout in
		// seconds.
		reply, err := conn.Do("SET", c.key, c.value, "NX", "EX", ttlStr)
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

		// Wait a while before retrying.
		select {
		case <-stopCh:
			// Lock acquisition was cancelled by our caller.
			return nil, nil
		case <-time.After(c.lockConf.leaderLockRetryInterval):
			// Timeout fired, so here we go again.
		}
	}

	// Set up a background listener to renew our lock and notice if it
	// goes away.
	leaderCh := make(chan struct{})
	go func() {
		conn := c.pool.Get()
		defer conn.Close()

		// Create our TTL-bumping script and try to load
		// it. Loading is optional because of how Redigo is
		// implemented.
		bumpTtlScript := redis.NewScript(1, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("EXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
`)
		_ = bumpTtlScript.Load(conn)

		// Renew our TTL periodically.
		for {
			time.Sleep(c.lockConf.leaderTTLRenewInterval)
			reply, err := bumpTtlScript.Do(conn, c.key, c.value, ttlStr)
			result, err := redis.Int(reply, err)
			if err != nil || result == 0 {
				close(leaderCh)
				if err == nil {
					err = fmt.Errorf("could not bump ttl")
				}
				c.logger.Printf("[WARN]: redis: lost lock: %v", err)
				return
			}
		}
	}()

	return leaderCh, nil
}

// Release the lock.
func (c *RedisLock) Unlock() error {
	conn := c.pool.Get()
	defer conn.Close()

	// Declare a Redis Lua script that we'll send to the server.  This
	// will delete our lock key, but only if it contains the value that
	// we expect.
	unlockScript := redis.NewScript(1, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

	// Run our unlock script.
	reply, err := unlockScript.Do(conn, c.key, c.value)
	deleted, err := redis.Int(reply, err)
	if err != nil {
		return err
	}
	if deleted == 0 {
		// I presume we ought to return an error here.
		return fmt.Errorf("redis: tried to unlock a lock we didn't own")
	}
	return nil
}

// Returns the value of the lock and if it is held.
func (c *RedisLock) Value() (bool, string, error) {
	conn := c.pool.Get()
	defer conn.Close()

	reply, err := conn.Do("GET", c.key)
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

// Testing: Simulate an expiration of our lock by deleting our key.
func (c *RedisLock) simulateExpiration() error {
	conn := c.pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", c.key)
	return err
}