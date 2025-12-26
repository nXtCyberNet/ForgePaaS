package redis

import (
	"models/message"
	"context"
	"fmt"
    "encoding/json"
	"github.com/redis/go-redis/v9"
)

func connect() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
	})

}

var rdb = connect()

func test() bool {
	_ , err := rdb.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("broken redis..")
		return false
	}
	return true

}

func startconsumer() {
	if test() != true {
		fmt.Println("redis failed")
		return 
	}
	for {
		msg, err := rdb.BRPop(context.Background(), 0, "create_queue","delete_queue").Result()
		if err != nil {
			continue
		}

		var job message.Job

		queue := msg[0]
		


	switch queue {
	case "create_queue":
			err := json.Unmarshal([]byte(msg[1]), &job)
			if err!=nil{
				fmt.Println(err)
			}
			//call the create logic 
		
		case "delete_queue":
			err :=json.Unmarshal([]byte(msg[1]), &job)
			if err!=nil{
				fmt.Println(err)
			}
			//call the delete logic 
		
	}



}
