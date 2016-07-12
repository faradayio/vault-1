package physical

import (
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
	path       string
	conn       *redis.Conn
	logger     *log.Logger
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
	return fmt.Errorf("RedisBackend Put unimplemented")
}

func (c *RedisBackend) Get(key string) (*Entry, error) {
	return nil, fmt.Errorf("RedisBackend Get unimplemented")
}

func (c *RedisBackend) Delete(key string) error {
	return fmt.Errorf("RedisBackend Delete unimplemented")
}

func (c *RedisBackend) List(prefix string) ([]string, error) {
	return nil, fmt.Errorf("RedisBackend List unimplemented")
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
