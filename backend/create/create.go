package create

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/network"
	appv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	label := map[string]string{"app": appname}

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
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: corev1.PodSpec{
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

							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("125m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},

							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8080),
									},
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

	err := Createnamespace(client, appname)
	if err != nil {
		return nil, err
	}

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
	err = DeleteNamespace(client, appname)
	if err != nil {
		return err
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
	err = DeleteNamespace(client, appname)
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

func Createnamespace(client kubernetes.Interface, appname string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: appname,
		},
	}

	_, err := client.CoreV1().
		Namespaces().
		Create(ctx, ns, metav1.CreateOptions{})

	// If namespace already exists, do NOT fail
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	return nil
}

func DeleteNamespace(client kubernetes.Interface, appname string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.CoreV1().
		Namespaces().
		Delete(ctx, appname, metav1.DeleteOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func NetworkPolicies(client kubernetes.Interface , appname string ) {
	client = client.NetworkingV1().NetworkPolicies(appname)

}
