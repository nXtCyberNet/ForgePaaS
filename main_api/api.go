package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/redis/go-redis/v9"
)

type create struct {
	GitRepo string `json:"gitrepo"`
	UserId  string `json:"userid"`
}

type delete struct {
	DepId string `json:"depid"`
}

func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func NewRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}

var rdb = NewRedis()

func createe(c *gin.Context) {
	var data create
	queue := "create_queue"

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid json",
		})
		return
	}
	if data.GitRepo == "" || data.UserId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing fields"})
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		c.JSON(500, gin.H{"error": "marshal failed"})
		return
	}

	err1 := rdb.LPush(context.Background(), queue, payload).Err()
	if err1 != nil {
		c.JSON(500, gin.H{"error": "redis error "})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})

}

func deletee(c *gin.Context) {
	var data delete
	queue := "delete_queue"

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid json",
		})
		return
	}

	payload, err := json.Marshal(data)
	if err != nil {
		c.JSON(500, gin.H{"error": "marshal failed"})
		return
	}

	if data.DepId != "" {

		err := rdb.LPush(context.Background(), queue, payload).Err()
		if err != nil {
			c.JSON(500, gin.H{"error": "marshel failed"})
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})

	}

}

func main() {
	r := gin.New()
	r.GET("/health", Health)

	r.POST("/create", createe)
	r.POST("/delete", deletee)

	r.Run(":8080")

}
