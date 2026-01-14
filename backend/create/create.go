package create

import (
	"context"
	"fmt"
	"time"

	appv1 "k8s.io/api/apps/v1" //
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func int64Ptr(i int64) *int64 {
	return &i
}
func int32Ptr(i int32) *int32 {
	return &i
}

func CreateDep(image_url string, depid string, appname string) *appv1.Deployment {
	maxSurge := intstr.FromInt(1)
	maxUnavailable := intstr.FromInt(0)
	label := map[string]string{"app": depid}
	dep := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appname,
			Namespace: appname,
		},
		Spec: appv1.DeploymentSpec{
			Replicas: int32Ptr(2),

			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Strategy: appv1.DeploymentStrategy{
				Type: appv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   appname,
					Labels: label,
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(120),

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

func DeplomentRunner(client kubernetes.Interface, dep *appv1.Deployment, appname string) (*appv1.Deployment, error) {

	create, err := client.AppsV1().Deployments(appname).Create(context.Background(), dep, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return create, nil

}

func InstDelete(client kubernetes.Interface, dynclient dynamic.Interface, namespace string, appname string) error {
	route := appname + "-route"
	ingressRouteRes := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes",
	}
	grace := int64(30)
	service := appname + "-service"
	err := client.AppsV1().
		Deployments(namespace).
		Delete(
			context.Background(),
			appname,
			metav1.DeleteOptions{
				GracePeriodSeconds: &grace,
			},
		)
	if err != nil {
		return err
	}
	errr := client.CoreV1().Services(appname).Delete(context.Background(), service, metav1.DeleteOptions{})
	if errr != nil {
		return errr
	}
	errrr := dynclient.Resource(ingressRouteRes).Delete(context.Background(), route, metav1.DeleteOptions{})
	if errrr != nil {
		return errrr
	}
	return nil

}

func Deletegracefully(client kubernetes.Interface, dynclient dynamic.Interface, namespace string, appname string) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	serviceName := appname + "-service"
	routeName := appname + "-route"

	ingressRouteRes := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes",
	}

	replicas := int32(0)
	_, err := client.AppsV1().
		Deployments(namespace).
		UpdateScale(
			ctx,
			appname,
			&autoscalingv1.Scale{
				Spec: autoscalingv1.ScaleSpec{Replicas: replicas},
			},
			metav1.UpdateOptions{},
		)
	if err != nil {
		return err
	}

	_ = dynclient.
		Resource(ingressRouteRes).
		Namespace(namespace).
		Delete(ctx, routeName, metav1.DeleteOptions{})

	_ = client.CoreV1().
		Services(namespace).
		Delete(ctx, serviceName, metav1.DeleteOptions{})

	err = client.AppsV1().
		Deployments(namespace).
		Delete(ctx, appname, metav1.DeleteOptions{})
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
