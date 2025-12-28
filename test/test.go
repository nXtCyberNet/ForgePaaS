package test

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/user"

	"k8s.io/client-go/tools/clientcmd"

	batchv1 "k8s.io/api/batch/v1"                 // For the Job itself
	corev1 "k8s.io/api/core/v1"                   // For the Pod/Container inside the Job
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // For Metadata (Names)
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func int32Ptr(i int32) *int32 {
	return &i
}

func expandPath(path string) string {
	if path[:2] == "~/" {
		u, _ := user.Current()
		return u.HomeDir + path[1:]
	}
	return path
}

func CreateClient(kubeconfigPath string) (kubernetes.Interface, error) {
	var kubeconfig *rest.Config

	if kubeconfigPath != " " {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			log.Println("failed:", err)

		}
		kubeconfig = config

	} else {
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Println("failed:", err)

		}
		kubeconfig = config

	}
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		panic(err)

	}
	return client, nil

}

func CreateJob(clientset kubernetes.Interface, imagee, commander string) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "builder" + imagee,
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(4),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "demo"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{

							Name:  imagee,
							Image: imagee,

							Command: []string{"sh", "-c", commander},
						},
					},
				},
			},
		},
	}
	fmt.Println("creating...")
	result, err := clientset.BatchV1().Jobs("default").Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		log.Println("filed", err)
	}
	fmt.Printf("Created Job %q.\n", result.GetObjectMeta().GetName())

}

func main() {
	kubeconfig := flag.String("filepath", "~/.kube/config", "kubeconfig file path ")
	image := flag.String("image", "ubuntu", "image of the job you want to use ")
	command := flag.String("command", "echo bhoo ", "command to run inside the container")

	flag.Parse()

	client, _ := CreateClient(expandPath(*kubeconfig))
	fmt.Println(client)
	CreateJob(client, *image, *command)

}
