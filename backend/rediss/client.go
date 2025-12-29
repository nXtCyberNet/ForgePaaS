package rediss

import (
	"context"
	"encoding/json"
	"fmt"
	"minihiroku/backend/models"

	"github.com/redis/go-redis/v9"
)

func Connect() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

}

func test(rdb *redis.Client) bool {
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("broken redis..")
		return false
	}
	return true

}

func CheckReady(rdb *redis.Client) ([]string, error) {
	msg, err := rdb.BRPop(context.Background(), 0, "buildqueue ").Result()
	if err != nil {
		return nil, err
	}
	return msg, nil

}

func StartConsumer(rdb *redis.Client) (*models.Create, error) {
	if test(rdb) != true {
		fmt.Println("redis failed")
		return nil, fmt.Errorf("redis stopped ")
	}
	for {
		msg, err := rdb.BRPop(context.Background(), 0, "create_queue", "delete_queue").Result()
		if err != nil {
			continue
		}

		var crr models.Create

		queue := msg[0]

		switch queue {
		case "create_queue":
			err := json.Unmarshal([]byte(msg[1]), &crr)
			if err != nil {
				return nil, err
			}
			return &crr, nil

		case "delete_queue":
			err := json.Unmarshal([]byte(msg[1]), &crr)
			if err != nil {
				return nil, err
			}
			return &crr, nil

		}

	}

}
