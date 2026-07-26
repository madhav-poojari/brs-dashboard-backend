package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Client struct{
	client *redis.Client
}

func NewRedisClient(RedisURL string) (Client, error) {
	url, err := redis.ParseURL(RedisURL)
	if err != nil {
		return Client{client: nil}, err
	}
	rdb := redis.NewClient(url)

	ctx := context.Background()

	pong, err := rdb.Ping(ctx).Result()
	fmt.Println(pong, err)
	if err != nil {
		return Client{client: nil}, err
	}
	return Client{client: rdb}, nil
}
