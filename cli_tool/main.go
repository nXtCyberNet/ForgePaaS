package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type CreatePayload struct {
	GitRepo string `json:"gitrepo"`
	UserId  string `json:"userid"`
	AppName string `json:"appname"`
}

type DeletePayload struct {
	UserID string `json:"userId"`
	DepId  string `json:"depID"`
}

type ConfigPayload struct {
	APIURL      string `json:"apiUrl"`
	DatabaseURL string `json:"databaseUrl"`
	UserID      string `json:"userID"`
}

func postJSON(url string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %s", resp.Status)
	}

	return nil
}

func CreateResource(baseURL, userID, repo string, appname string) error {
	payload := CreatePayload{
		GitRepo: repo,
		UserId:  userID,
		AppName: appname,
	}

	url := strings.TrimSuffix(baseURL, "/") + "/create"
	return postJSON(url, payload)
}

func DeleteResource(baseURL, userID, depID string) error {
	payload := DeletePayload{
		UserID: userID,
		DepId:  depID,
	}
	url := strings.TrimSuffix(baseURL, "/") + "/delete"
	return postJSON(url, payload)
}

func askInput(reader *bufio.Reader, question string) string {
	for {
		fmt.Print(question)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text != "" {
			return text
		}
		fmt.Println("Value cannot be empty, try again.")
	}
}

func GenerateUserID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"

	id := make([]byte, 10)
	for i := range id {
		id[i] = chars[rand.Intn(len(chars))]
	}
	return string(id)
}

func getConfigPath() string {

	return filepath.Join(".", "configs", "config.json")
}

func RunConfigSetup() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Config setup")
	fmt.Println("------------")

	apiURL := askInput(reader, "Enter API URL (e.g., http://localhost:8080): ")
	dbURL := askInput(reader, "Enter Database URL: ")
	userid := GenerateUserID()

	cfg := ConfigPayload{
		APIURL:      apiURL,
		DatabaseURL: dbURL,
		UserID:      userid,
	}

	path := getConfigPath()
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling config: %v\n", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Config saved to %s\n", path)
	fmt.Printf("Generated User ID: %s\n", userid)
}

func LoadConfig() (ConfigPayload, error) {
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return ConfigPayload{}, err
	}

	var cfg ConfigPayload
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func HandleLogs(cfg ConfigPayload) {
	logsCmd := flag.NewFlagSet("logs", flag.ExitOnError)
	app := logsCmd.String("app", "", "App name to stream logs for")

	logsCmd.Parse(os.Args[2:])

	if *app == "" {
		fmt.Println("Error: missing -app flag")
		logsCmd.PrintDefaults()
		return
	}

	// 1. Convert HTTP URL to WS URL
	// e.g., "http://localhost:8080" -> "ws://localhost:8080/logs"
	parsedURL, _ := url.Parse(cfg.APIURL)
	scheme := "ws"
	if parsedURL.Scheme == "https" {
		scheme = "wss"
	}

	u := url.URL{
		Scheme:   scheme,
		Host:     parsedURL.Host,
		Path:     "/logs",
		RawQuery: "app=" + *app,
	}

	fmt.Printf("Connecting to log stream for %s at %s...\n", *app, u.String())

	// 2. Connect to WebSocket
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	// 3. Handle Ctrl+C gracefully
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})

	// 4. Read Loop (Background)
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				// Determine if it was a clean close or error
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					fmt.Printf("Connection lost: %v\n", err)
				}
				return
			}
			// Print the log line to the terminal
			fmt.Println(string(message))
		}
	}()

	// 5. Wait for Interrupt
	select {
	case <-interrupt:
		fmt.Println("\nDisconnecting...")
		// Cleanly close the connection
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			return
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	case <-done:
		fmt.Println("Server closed connection.")
	}
}

func HandleCLI(cfg ConfigPayload) {
	if len(os.Args) < 2 {
		fmt.Println("Expected 'create' or 'delete' or 'logs' subcommand")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		HandleCreate(cfg)
	case "delete":
		HandleDelete(cfg)
	case "logs":
		HandleLogs(cfg)
	default:
		if os.Args[1] == "-config" || os.Args[1] == "--config" {
			return
		}
		fmt.Println("Unknown command:", os.Args[1])
		fmt.Println("Usage: mycli [create|delete] [flags]")
	}
}

func HandleCreate(cfg ConfigPayload) {
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	repo := createCmd.String("repo", "", "Github repo ID or URL")
	appname := createCmd.String("app", "", "appname")

	createCmd.Parse(os.Args[2:])

	if *repo == "" {
		fmt.Println("Error: missing -repo flag")
		createCmd.PrintDefaults()
		return
	}
	if *appname == "" {
		fmt.Println("Error: missing -app flag")
		createCmd.PrintDefaults()
		return
	}

	fmt.Printf("Deploying repo: %s , %s for user: %s...\n", *repo, *appname, cfg.UserID)

	err := CreateResource(cfg.APIURL, cfg.UserID, *repo, *appname)
	if err != nil {
		fmt.Println("Create failed:", err)
		return
	}

	fmt.Println("Create success!")
}

func HandleDelete(cfg ConfigPayload) {
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	app := deleteCmd.String("app", "", "App ID to delete")

	deleteCmd.Parse(os.Args[2:])

	if *app == "" {
		fmt.Println("Error: missing -app flag")
		deleteCmd.PrintDefaults()
		return
	}

	fmt.Printf("Deleting app: %s for user: %s...\n", *app, cfg.UserID)

	err := DeleteResource(cfg.APIURL, cfg.UserID, *app)
	if err != nil {
		fmt.Println("Delete failed:", err)
		return
	}

	fmt.Println("Delete success!")
}

func main() {

	setupFlag := flag.Bool("config", false, "Run configuration setup")
	flag.Parse()

	if *setupFlag {
		RunConfigSetup()
		return
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Println("Error loading config. Have you run 'mycli -config' yet?")
		fmt.Println("Details:", err)
		os.Exit(1)
	}

	HandleCLI(cfg)
}
