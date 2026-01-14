package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"minihiroku/backend/create"
	"minihiroku/backend/image"
	"minihiroku/backend/models"
	"minihiroku/backend/rediss"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	dynclient, err := image.NewDynamicClient(os.Getenv("kubeconfigPath"))
	if err != nil {
		log.Println(err)

	}
	log.Println("k8s connected")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {

		consumer, err := rediss.StartConsumer(ctx, rds)
		if err != nil || consumer == nil {
			log.Println("consumer not ready")
			time.Sleep(2 * time.Second)
			continue
		}
		if consumer.Queue == "create" {
			log.Printf(" finded the payload appname : %s , depid %s , gitrepo : %s", consumer.Create.AppName, consumer.Create.DepId, consumer.Create.GitRepo)

			go DeploymentPipeline(dynclient, client, consumer.Create, rds)

		} else if consumer.Queue == "delete" {
			log.Printf("payload: %s , %s , force : %t", consumer.Delete.UserID, consumer.Delete.AppName, consumer.Delete.Force)
			go deleteapp(dynclient, client, consumer.Delete, rds)

		} else {

		}

	}
}

func DeploymentPipeline(dynclient dynamic.Interface, client kubernetes.Interface, consumer *models.Create, rds *redis.Client) {

	logsend := func(msg string) {
		rediss.PublishLog(rds, consumer.AppName, msg)
	}

	defer func() {
		if r := recover(); r != nil {
			logsend(fmt.Sprintf("⚠️ CRITICAL ERROR: %v", r))
		}
	}()
	logsend("Initializing build job...")
	job, apptag := image.JobObject(consumer.GitRepo, consumer.AppName, consumer.DepId, os.Getenv("REGISTORY_URL"))
	log.Println("job created ")
	log.Println(apptag)

	runnn, err := image.JobRunner(client, job)
	if err != nil {
		logsend(fmt.Sprintf("❌ Job creation failed: %v", err))
		return
	}
	logsend(fmt.Sprintf("Build Job started (Pod: %s)", runnn.Name))

	go func() {
		time.Sleep(2 * time.Second)
		image.LogsGiver(client, runnn.Name, job.Namespace, rds, consumer.AppName)
	}()

	logsend("Waiting for build to complete...")
	check, err := rediss.CheckReady(rds, consumer.AppName)
	if err != nil || len(check) < 2 {
		logsend("❌ Error receiving completion signal from builder")
		return
	}

	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(check[1]), &msg); err != nil {
		log.Println("invalid json:", err)

		return
	}
	log.Println("got the image ready signal  ")
	log.Println(check)
	apptag = os.Getenv("REGISTORY_CLUSTER_IP") + "/" + apptag

	if msg["status"] == "ready" {
		logsend("Build successful. Starting deployment...")
		dep := create.CreateDep(apptag, consumer.DepId, consumer.AppName)
		runn, err := create.DeplomentRunner(client, dep, consumer.AppName)

		if err != nil {
			logsend(fmt.Sprintf("❌ Deployment failed: %v", err))
			return
		}
		logsend(fmt.Sprintf("Deployment created (UID: %s)", runn.UID))

		errr := create.CreateService(client, runn.Namespace, consumer.AppName)

		if errr != nil {
			log.Println(errr)
			logsend(fmt.Sprintf("❌ Service creation failed: %v", err))
			return
		}
		logsend("Service exposed internally.")
		log.Println("service created ")
		time.Sleep(10 * time.Second)
		rout := create.CreateRoute(dynclient, consumer.AppName, os.Getenv("DOMAIN"), runn.Namespace)
		if rout != nil {
			logsend(fmt.Sprintf("❌ Route creation failed: %v", rout))
			return
		}
		log.Println("route created ")
		finalURL := fmt.Sprintf("http://%s.%s", consumer.AppName, os.Getenv("DOMAIN"))

		log.Println("deployment info ", runn.Name, runn.Namespace, runn.UID)
		logsend(fmt.Sprintf("🎉 SUCCESS! Your app is live at: %s", finalURL))

	}

}

func deleteapp(dynclient dynamic.Interface, client kubernetes.Interface, consumer *models.Delete, rds *redis.Client) {
	force := consumer.Force
	appname := consumer.AppName

	if force == true {
		err := create.InstDelete(client, dynclient, appname, name, appname)
		if err != nil {
			log.Println(err)
		}

	} else {
		err := create.Deletegracefully(client, dynclient, appname, name, appname)
		if err != nil {
			log.Println(err)
		}

	}
}
