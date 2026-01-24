package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func LoadEnv() error {
	return godotenv.Load(".env")
}

var rdb *redis.Client

func NewRedis(ip string, pass string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: ip, Password: pass})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("⚠️ redis not ready yet: %v", err)

	}
	for i := 0; i < 10; i++ {
		if err := rdb.Ping(context.Background()).Err(); err == nil {
			log.Println("✅ redis connected")
			return rdb
		}
		time.Sleep(2 * time.Second)
	}

	log.Println("⚠️ redis unavailable, continuing without it")

	return rdb
}

type create struct {
	GitRepo string `json:"gitrepo"`
	AppName string `json:"appname"`
	UserId  string `json:"userid"`
	DepID   string `json:"depid"`
}

type delete struct {
	Appname string `json:"appname"`
	UserId  string `json:"userid"`
	Force   bool   `json:"force"`
}

func GenerateDepID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 8)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return ""
		}
		result[i] = chars[num.Int64()]
	}
	return "dep-" + string(result)
}

func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func createe(c *gin.Context) {
	var data create
	queue := "create_queue"

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if data.GitRepo == "" || data.UserId == "" || data.AppName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing fields"})
		return
	}

	data.DepID = GenerateDepID()

	payload, err := json.Marshal(data)
	if err != nil {
		c.JSON(500, gin.H{"error": "marshal failed"})
		return
	}

	err1 := rdb.LPush(context.Background(), queue, payload).Err()
	if err1 != nil {
		c.JSON(500, gin.H{"error": "redis error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"depid":  data.DepID,
	})
}

func deletee(c *gin.Context) {
	var data delete
	queue := "delete_queue"

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if data.Appname == "" || data.UserId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing fields"})
		return
	}

	payload, err := json.Marshal(data)
	if err != nil {
		c.JSON(500, gin.H{"error": "marshal failed"})
		return
	}

	err = rdb.LPush(context.Background(), queue, payload).Err()
	if err != nil {
		c.JSON(500, gin.H{"error": "redis error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func streamLogs(c *gin.Context) {
	appName := c.Query("app")
	if appName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'app' query parameter"})
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("❌ WebSocket Upgrade Failed:", err)
		return
	}
	defer ws.Close()

	log.Printf("✅ Client connected via WebSocket for: %s", appName)

	streamRedisToWebSocket(ws, rdb, appName)
}

func streamRedisToWebSocket(ws *websocket.Conn, rds *redis.Client, appName string) {
	ctx := context.Background()
	channelName := "logs:" + appName

	pubsub := rds.Subscribe(ctx, channelName)
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		log.Printf("❌ Redis Subscription failed for %s: %v", appName, err)
		ws.WriteMessage(websocket.TextMessage, []byte("[SYSTEM] Error connecting to log source"))
		return
	}

	ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("[SYSTEM] Connected to log stream for %s...", appName)))

	ch := pubsub.Channel()
	for msg := range ch {
		err := ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		if err != nil {
			log.Printf("👋 Client disconnected from %s", appName)
			return
		}
	}
}

func main() {
	rdb = NewRedis(
		os.Getenv("REDIS_URL"),
		os.Getenv("REDIS_PASS"),
	)
	r := gin.Default()
	LoadEnv()

	r.GET("/health", Health)
	r.POST("/create", createe)
	r.POST("/delete", deletee)
	r.GET("/logs", streamLogs)

	r.Run(":8080")
}
