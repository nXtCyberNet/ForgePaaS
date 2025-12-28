package create

import (
	"context"
	"log"
	"minihiroku/backend/image"
	"os"

	"github.com/joho/godotenv"

	appv1 "k8s.io/api/apps/v1" // For Metadata (Names)
	// For the Job itself
	corev1 "k8s.io/api/core/v1" // For the Pod/Container inside the Job
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func int32Ptr(i int32) *int32 {
	return &i
}

func takedata(value string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")

	}
	a := os.Getenv(value)
	return a
}

func CreateDep(image_url string, depid string) *appv1.Deployment {
	dep := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depid,
			Namespace: "runners",
		},
		Spec: appv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": depid},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: depid,
				},
				Spec: v1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  " ",
							Image: image_url,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}
	return dep
}

func main() {
	var kubeconfigPath string
	client, err := image.CreateClient(takedata(kubeconfigPath))
	if err != nil {
		log.Println(err)
	}

	result, err := client.BatchV1().Jobs("builder").Create(context.Background(), image.jobobject(), metav1.CreateOptions{})
	if err != nil {
		log.Println("")
	}
	log.Printf("job created info %s", &result.ObjectMeta)

	dep := CreateDep("bro ", "depid")

	create, err := client.AppsV1().Deployments("runner").Create(context.Background(), dep, metav1.CreateOptions{})
	if err != nil {
		log.Println(err)
	}
	log.Println(create)

}
