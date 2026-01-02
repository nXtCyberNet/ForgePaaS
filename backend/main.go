package main

import (
	"encoding/json"
	"log"
	"minihiroku/backend/create"
	"minihiroku/backend/image"
	"minihiroku/backend/rediss"
	"os"

	"github.com/joho/godotenv"
)

func LoadEnv() error {
	return godotenv.Load("backend/.env")
}

func main() {
	rds := rediss.Connect()
	log.Println("redis connected ")
	err := LoadEnv()
	if err != nil {
		log.Println("env not loaded ")
	}

	client, err := image.CreateClient(os.Getenv("kubeconfigPath"))
	if err != nil {
		log.Println("k8s not connected ")
	}

	consumer, err := rediss.StartConsumer(rds)
	if err != nil {
		log.Println(err)
	}
	log.Printf(" finded the payload appname : %s , depid %s , gitrepo : %s", consumer.AppName, consumer.DepId, consumer.GitRepo)

	log.Println("k8s connected")
	log.Println(client)

	job, apptag := image.JobObject(consumer.GitRepo, consumer.AppName, consumer.DepId, os.Getenv("REGISTORY_URL"))
	log.Println("job created ")
	log.Println(apptag)

	runnn, err := image.JobRunner(client, job)
	if err != nil {
		log.Fatal("job creation failed:", err)
	}

	log.Println("jo info ", runnn.Name, runnn.Namespace, runnn.CreationTimestamp, runnn.UID)

	err = image.LogsGiver(client, apptag, "builder")

	check, err := rediss.CheckReady(rds, consumer.AppName)
	if err != nil {
		log.Println(err)
	}
	var msg map[string]interface{}
	json.Unmarshal([]byte(check[1]), &msg)
	log.Println("got the image ready signal  ")
	log.Println(check)
	apptag = os.Getenv("REGISTORY_CLUSTER_IP") + "/" + apptag

	if msg["status"] == "ready" {
		dep := create.CreateDep(apptag, consumer.DepId)
		runn, err := create.DeplomentRunner(client, dep)
		if err != nil {
			log.Println(err)
		}
		log.Println("deployment info ", runn.Name, runn.Namespace, runn.UID)

	}

}
