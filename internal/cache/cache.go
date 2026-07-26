package cache

import (
	"context"
	"time"
)

func (c Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	err := c.client.Set(ctx, key, value, ttl).Err()
	return err
}

func (c Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

func (c Client) Delete(ctx context.Context, keys ...string) error {
	err := c.client.Del(ctx, keys...).Err()
	return err
}
