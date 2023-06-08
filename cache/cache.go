package cache

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

type Value struct {
	Key   string
	Value []byte
}

type memoryCache struct {
	sync.RWMutex
	Values map[string]Value
}

var (
	Cache cache = newMemoryCache()
)

type cache interface {
	Get(key string, val interface{}) error
	Set(key string, val interface{}) error
	Delete(key string) error
}

func newMemoryCache() *memoryCache {
	return &memoryCache{
		Values: make(map[string]Value),
	}
}

func (c *memoryCache) Get(key string, val interface{}) error {
	c.RLock()
	defer c.RUnlock()

	v, ok := c.Values[key]
	if !ok {
		return errors.New("not found")
	}
	return json.Unmarshal(v.Value, val)
}

func (c *memoryCache) Set(key string, val interface{}) error {
	c.Lock()
	defer c.Unlock()

	b, err := json.Marshal(val)
	if err != nil {
		return err
	}

	c.Values[key] = Value{
		Key:   key,
		Value: b,
	}

	return nil
}

func (c *memoryCache) Delete(key string) error {
	c.Lock()
	defer c.Unlock()
	delete(c.Values, key)
	return nil
}

func Get(key string, val interface{}) error {
	return Cache.Get(key, val)
}

func Set(key string, val interface{}) error {
	return Cache.Set(key, val)
}

func Delete(key string) error {
	return Cache.Delete(key)
}

func Init(addr string) error {
	if strings.HasPrefix(addr, "redis") {
		if c, err := newRedis(addr); err != nil {
			return err
		} else {
			Cache = c
		}
		return nil
	}
	Cache = newMemoryCache()
	return nil
}
