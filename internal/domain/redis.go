package domain

import (
	"context"

	"github.com/go-redis/redis"
	"github.com/labstack/gommon/log"
	"go.uber.org/zap"
)

func InitRedis(host, port string) *redis.Client {
	//url := "redis://user:password@localhost:6379/0?protocol=3"
	ctx := context.Background()

	opts, err := redis.ParseURL("redis://localhost:6379")
	if err != nil {
		log.Fatal(ctx, "unable to parse redis url",
			zap.Error(err))

		return nil
	}

	client := redis.NewClient(opts)

	// Ping the Redis server to check the connection
	_, err = client.Ping().Result()
	if err != nil {
		log.Fatal(ctx, "unable to connect to Redis server",
			zap.Error(err))

		return nil
	}
	// Enable keyspace notifications for expired events
	err = client.ConfigSet("notify-keyspace-events", "Ex").Err()
	if err != nil {
		log.Error(ctx, "failed to set notify-keyspace-events", zap.Error(err))
	} else {
		log.Info(ctx, "Keyspace notifications for expiration enabled")
	}

	return client
}
