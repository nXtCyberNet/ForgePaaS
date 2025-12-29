package image

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	batchv1 "k8s.io/api/batch/v1" // For the Job itself
	corev1 "k8s.io/api/core/v1"   // For the Pod/Container inside the Job
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // For Metadata (Names)
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func int32Ptr(i int32) *int32 {
	return &i
}

func CreateClient(kubeconfigPath string) (kubernetes.Interface, error) {
	var kubeconfig *rest.Config

	if kubeconfigPath != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, err

		}
		kubeconfig = config

	} else {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err

		}
		kubeconfig = config

	}
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return client, nil

}

func secretstaker(client kubernetes.Interface, name string) *v1.Secret {

	secret, err := client.CoreV1().Secrets("registory").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		log.Println(err)
	}
	return secret

}

func JobRunner(client kubernetes.Interface, job *batchv1.Job) (*batchv1.Job, error) {
	result, err := client.BatchV1().Jobs("builder").Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return result, nil

}

func JobObject(giturl string, appName string, depid string) (*batchv1.Job, string) {
	apptag := appName + depid

	cnbCmd := fmt.Sprintf(
		`/cnb/lifecycle/creator \
    -app=/workspace \
    -image=%s \
    -skip-restore=false`,
		apptag,
	)

	var payload = fmt.Sprintf(
		`{"status":"ready","app":"%s","timestamp":%d}`,
		appName,
		time.Now().Unix())

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-" + appName,
			Namespace: "builder",
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(2),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "demo"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []v1.Volume{
						{
							Name: "workspace",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []v1.Container{
						{
							Name:    "pullrepo",
							Image:   "alpine/git",
							Command: []string{"sh", "-c", "git clone " + giturl + " /workspace"},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
							},
						},
						{
							Name:    "cnd-binary",
							Image:   "paketobuildpacks/builder-jammy-base:latest",
							Command: []string{"sh", "-c", cnbCmd},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "notifier",
							Image:   "redis:alpine",
							Command: []string{"redis-cli", "-h", "redis-service", "RPUSH", fmt.Sprintf("status:%s", appName), payload},

							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
							},
						},
					},
				},
			},
		},
	}
	return job, apptag
}
