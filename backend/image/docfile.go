package image

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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

func LogsGiver(client kubernetes.Interface, jobname string, namespace string, rds *redis.Client, appname string) {
	ctx := context.Background()
	channelName := "logs:" + appname

	publish := func(msg string) {
		if err := rds.Publish(ctx, channelName, msg).Err(); err != nil {
			log.Printf("redis publish failed: %v", err)
		}
	}

	publish(fmt.Sprintf("[SYSTEM] Waiting for build pod for job: %s...", jobname))

	var podName string
	for {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobname),
		})
		if err != nil {
			log.Printf("Error listing pods: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if len(pods.Items) > 0 {
			pod := pods.Items[0]

			if pod.Status.Phase != corev1.PodUnknown {
				podName = pod.Name
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	publish(fmt.Sprintf("[SYSTEM] Found Pod: %s. preparing log stream...", podName))

	containers := []string{"pullrepo", "cnd-binary", "notifier"}

	for _, containerName := range containers {

		for {
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			if pod.Status.Phase == corev1.PodFailed {
				publish(fmt.Sprintf("[SYSTEM] ❌ Pod failed before %s could start", containerName))
				return
			}

			isReady := false

			for _, status := range pod.Status.InitContainerStatuses {
				if status.Name == containerName {

					if status.State.Running != nil || status.State.Terminated != nil {
						isReady = true
					}

					if status.State.Waiting != nil {

						reason := status.State.Waiting.Reason
						if reason != "" {
							publish(fmt.Sprintf("[SYSTEM] ❌ %s failed: %s", containerName, reason))
							return
						}

					}
				}
			}

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

			time.Sleep(1 * time.Second)
		}

		publish(fmt.Sprintf("[SYSTEM] --- Starting Step: %s ---", containerName))

		req := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: containerName,
			Follow:    true,
		})

		stream, err := req.Stream(ctx)
		if err != nil {

			log.Printf("Warning: Could not open stream for %s (it might be done): %v", containerName, err)
			continue
		}

		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 1024), 1024*1024)

		for scanner.Scan() {
			logLine := scanner.Text()
			formattedLog := fmt.Sprintf("[%s] %s", strings.ToUpper(containerName), logLine)
			rds.Publish(ctx, channelName, formattedLog)
		}
		stream.Close()
	}

	publish("[SYSTEM] Build Job Logs Finished.")
}

func JobObject(giturl string, appname string, depid string, registry_url string) (*batchv1.Job, string) {
	apptag := fmt.Sprintf("%s/%s:%s", registry_url, appname, depid)
	image := fmt.Sprintf("%s:%s", appname, depid)
	cacheTag := fmt.Sprintf("%s/%s:cache", registry_url, appname)
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
		appname,
		time.Now().Unix())

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-" + appname + depid,
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
							Command: []string{"redis-cli", "-h", "redis.default.svc.cluster.local", "RPUSH", fmt.Sprintf("status:%s", appname), payload},

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
