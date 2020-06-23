package database

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/go-redis/redis"
	"time"
)

const (
	CacheStorageKey = "cache:%s:%s"
	CacheVersionKey = "cache:versions"

	versionLimit = 2

	errorInterfaceCast = "unable to cast interface to object %s"
)

type CacheInterface interface {
	Set(string, interface{}, time.Duration) error
	Get(string, interface{}) error
	Delete(string) error
	FlushAll()
	CleanOldestVersion() error
}

type Cache struct {
	redis   redis.Cmdable
	version string
}

func NewCacheRedis(r redis.Cmdable, version string) (*Cache, error) {
	result := r.ZAdd(CacheVersionKey, redis.Z{Member: version, Score: float64(time.Now().UnixNano())})

	if result.Err() != nil {
		return nil, result.Err()
	}

	return &Cache{redis: r, version: version}, nil
}

func (c *Cache) Set(key string, value interface{}, duration time.Duration) error {
	var network bytes.Buffer

	enc := gob.NewEncoder(&network)
	err := enc.Encode(value)

	if err != nil {
		return err
	}

	if err := c.redis.Set(c.getStorageKey(key), network.Bytes(), duration).Err(); err != nil {
		return err
	}

	return nil
}

func (c *Cache) Get(key string, obj interface{}) error {
	b, err := c.redis.Get(c.getStorageKey(key)).Bytes()

	if err != nil {
		return err
	}

	var network bytes.Buffer
	network.Write(b)

	dec := gob.NewDecoder(&network)

	if err = dec.Decode(obj); err != nil {
		return fmt.Errorf(errorInterfaceCast, err.Error())
	}

	return nil
}

func (c *Cache) Delete(key string) error {
	return c.redis.Del(c.getStorageKey(key)).Err()
}

func (c *Cache) FlushAll() {
	c.redis.FlushAll()
}

func (c *Cache) CleanOldestVersion() error {
	res := c.redis.ZRevRange(CacheVersionKey, 0, -1)

	if res.Err() != nil {
		return res.Err()
	}

	if len(res.Val()) <= versionLimit {
		return nil
	}

	for _, val := range res.Val()[versionLimit:] {
		if err := c.cleanVersion(val); err != nil {
			return err
		}

		c.redis.ZRem(CacheVersionKey, val)
	}

	return nil
}

func (c *Cache) cleanVersion(version string) error {
	var cursor uint64
	var limit int64 = 100
	var err error

	for {
		var keys []string
		keys, cursor, err = c.redis.Scan(cursor, fmt.Sprintf(CacheStorageKey, version, "*"), limit).Result()

		if err != nil {
			return err
		}

		if len(keys) > 0 {
			res := c.redis.Unlink(keys...)

			if res.Err() != nil {
				return err
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}

func (c Cache) getStorageKey(key string) string {
	return fmt.Sprintf(CacheStorageKey, c.version, key)
}
