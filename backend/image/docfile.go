package image

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func NewDynamicClient(kubeconfigPath string) (dynamic.Interface, error) {

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return dynClient, nil
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

func LogsGiver(client kubernetes.Interface, jobname string, namespace string) error {
	ctx := context.Background()

	var podName string
	fmt.Printf("Waiting for pod creation for job: %s...\n", jobname)

	for {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobname),
		})
		if err != nil {
			return fmt.Errorf("error listing pods: %v", err)
		}

		if len(pods.Items) > 0 {

			pod := pods.Items[0]

			if pod.Status.Phase != corev1.PodPending {
				podName = pod.Name
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("Found Pod: %s. Starting log stream...\n", podName)

	containers := []string{"pullrepo", "cnd-binary", "notifier"}

	for _, containerName := range containers {
		fmt.Printf("\n--- [Logs: %s] ---\n", containerName)

		for {
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				log.Println("Error getting pod status:", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// Check status of InitContainers and Containers
			isReady := false

			// Check Init Containers
			for _, status := range pod.Status.InitContainerStatuses {
				if status.Name == containerName {
					// It's ready if it's Running or Terminated (finished)
					if status.State.Running != nil || status.State.Terminated != nil {
						isReady = true
					}
					// If it's Waiting with "ErrImagePull" or "CrashLoop", we should probably break/log
					if status.State.Waiting != nil && status.State.Waiting.Reason == "ErrImagePull" {
						return fmt.Errorf("container %s failed to pull image", containerName)
					}
				}
			}
			// Check Main Containers (for 'notifier')
			for _, status := range pod.Status.ContainerStatuses {
				if status.Name == containerName {
					if status.State.Running != nil || status.State.Terminated != nil {
						isReady = true
					}
				}
			}

			if isReady {
				break
			}

			// If the previous container failed, this one will never start.
			// You might want to add logic here to check if the Pod Failed.
			if pod.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("pod failed before container %s could start", containerName)
			}

			time.Sleep(1 * time.Second)
		}

		// 4. STREAM
		req := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: containerName,
			Follow:    true, // Follow until the container exits
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			log.Printf("Error opening stream for %s: %v", containerName, err)
			continue
		}

		// Copy to stdout
		_, err = io.Copy(os.Stdout, stream)
		stream.Close() // Don't defer inside a loop, close explicitly
		if err != nil {
			log.Printf("Error reading logs from %s: %v", containerName, err)
		}
	}

	fmt.Println("\n--- Build Job Finished ---")
	return nil
}

func JobObject(giturl string, appName string, depid string, registry_url string) (*batchv1.Job, string) {
	apptag := fmt.Sprintf("%s/%s:%s", registry_url, appName, depid)
	image := fmt.Sprintf("%s:%s", appName, depid)
	cacheTag := fmt.Sprintf("%s/%s:cache", registry_url, appName)
	runImage := "paketobuildpacks/run-jammy-base:latest"

	cnbCmd := fmt.Sprintf(
		"/cnb/lifecycle/creator "+
			"-app=/workspace "+
			"-cache-image=%s "+
			"-run-image=%s "+
			"-skip-restore=false "+
			"%s "+
			"2>&1",
		cacheTag, runImage, apptag,
	)

	log.Println(cnbCmd)

	var payload = fmt.Sprintf(
		`{"status":"ready","app":"%s","timestamp":%d}`,
		appName,
		time.Now().Unix())

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-" + appName + depid,
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
							Name:            "cnd-binary",
							Image:           "paketobuildpacks/builder-jammy-base:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "CNB_PLATFORM_API", Value: "0.11"},
							},
							Command: []string{"/bin/sh", "-c", cnbCmd},

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
							Command: []string{"redis-cli", "-h", "redis.default.svc.cluster.local", "RPUSH", fmt.Sprintf("status:%s", appName), payload},

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
	return job, image
}
