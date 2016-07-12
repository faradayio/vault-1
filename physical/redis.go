package physical

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

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
	return nil, fmt.Errorf("RedisBackend LockWith unimplemented")
}

//type Backend interface {
//	// Put is used to insert or update an entry
//	Put(entry *Entry) error
//
//	// Get is used to fetch an entry
//	Get(key string) (*Entry, error)
//
//	// Delete is used to permanently delete an entry
//	Delete(key string) error
//
//	// List is used ot list all the keys under a given
//	// prefix, up to the next prefix.
//	List(prefix string) ([]string, error)
//}
//
//type HABackend interface {
//	// LockWith is used for mutual exclusion based on the given key.
//	LockWith(key, value string) (Lock, error)
//}
