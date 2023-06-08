package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

type redisCache struct {
	client redis.UniversalClient
}

func newRedis(addr string) (*redisCache, error) {
	if len(addr) == 0 {
		addr = "redis://127.0.0.1:6379"
	}
	redisOptions, err := redis.ParseURL(addr)
	if err != nil {
		return nil, err
	}
	return &redisCache{redis.NewClient(redisOptions)}, nil
}

func (c *redisCache) Get(key string, val interface{}) error {
	b, err := c.client.Get(context.TODO(), key).Bytes()
	if err != nil && err == redis.Nil {
		return errors.New("not found")
	} else if err != nil {
		return err
	}

	return json.Unmarshal(b, val)
}

func (c *redisCache) Set(key string, val interface{}) error {
	return c.client.Set(context.TODO(), key, val, time.Duration(0)).Err()
}

func (c *redisCache) Delete(key string) error {
	return c.client.Del(context.TODO(), key).Err()
}
