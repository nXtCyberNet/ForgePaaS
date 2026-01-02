package create

import (
	"context"
	"fmt"

	appv1 "k8s.io/api/apps/v1" // For Metadata (Names)
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	// For the Job itself
	corev1 "k8s.io/api/core/v1" // For the Pod/Container inside the Job
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func int32Ptr(i int32) *int32 {
	return &i
}

func CreateDep(image_url string, depid string) *appv1.Deployment {
	label := map[string]string{"app": depid}
	dep := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depid,
			Namespace: "runners",
		},
		Spec: appv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   depid,
					Labels: label,
				},
				Spec: v1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "dep",
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

func CreateService(client kubernetes.Interface, namespace string, appname string) error {

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appname + "-service",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": appname,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	_, err := client.CoreV1().
		Services(namespace).
		Create(context.Background(), service, metav1.CreateOptions{})

	return err
}

func DeplomentRunner(client kubernetes.Interface, dep *appv1.Deployment) (*appv1.Deployment, error) {

	create, err := client.AppsV1().Deployments("runners").Create(context.Background(), dep, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return create, nil

}

func DeleteDeployment(client kubernetes.Interface, namespace string, name string) error {
	grace := int64(30)
	err := client.AppsV1().
		Deployments(namespace).
		Delete(
			context.Background(),
			name,
			metav1.DeleteOptions{
				GracePeriodSeconds: &grace,
			},
		)

	if err != nil {
		return err
	}
	return nil

}

func CreateRoute(client dynamic.Interface, appname string, domain string, namespace string) error {
	domain = appname + "." + domain

	ingressRouteRes := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes",
	}

	route := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      appname + "-route",
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				// "entryPoints": []string{"web"},
				"routes": []map[string]interface{}{
					{
						"match": fmt.Sprintf("Host(`%s`)", domain),
						"kind":  "Rule",
						"services": []map[string]interface{}{
							{
								"name": appname + "-service",
								"port": 8080,
							},
						},
					},
				},
			},
		},
	}

	route, err := client.Resource(ingressRouteRes).Namespace(namespace).Create(context.TODO(), route, metav1.CreateOptions{})
	return err
}
