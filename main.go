package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const tenantKubeconfigSecretKey = "kubeconfig"

type SecretController struct {
	clientset           kubernetes.Interface
	getTenantClientFunc func(secret *corev1.Secret) (kubernetes.Interface, error)
}

func NewSecretController(clientset kubernetes.Interface) *SecretController {
	return &SecretController{
		clientset: clientset,
		getTenantClientFunc: func(secret *corev1.Secret) (kubernetes.Interface, error) {
			kubeconfigBytes, ok := secret.Data[tenantKubeconfigSecretKey]
			if !ok {
				return nil, ErrMissingKubeconfig
			}

			config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
			if err != nil {
				return nil, err
			}

			return kubernetes.NewForConfig(config)
		},
	}
}

func (c *SecretController) Run(stopCh <-chan struct{}) {
	factory := informers.NewSharedInformerFactory(c.clientset, time.Minute*10)
	secretInformer := factory.Core().V1().Secrets().Informer()

	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleSecretAdd,
	})

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	<-stopCh
	log.Println("Shutting down controller")
}

func (c *SecretController) handleSecretAdd(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		log.Println("Failed to cast object to Secret")
		return
	}

	log.Printf("Secret added: %s/%s\n", secret.Namespace, secret.Name)

	// Get tenant kubeconfig from same namespace
	kubeconfigSecret, err := c.clientset.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), "tenant-kubeconfig", metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get tenant kubeconfig secret: %v\n", err)
		return
	}

	tenantClient, err := c.getTenantClientFunc(kubeconfigSecret)
	if err != nil {
		log.Printf("Failed to create tenant client: %v\n", err)
		return
	}

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Data: secret.Data,
		Type: secret.Type,
	}

	_, err = tenantClient.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), newSecret, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create Secret in tenant cluster: %v\n", err)
		return
	}

	log.Printf("Successfully synced Secret to tenant cluster: %s/%s", newSecret.Namespace, newSecret.Name)
}

var ErrMissingKubeconfig = &ErrorString{"missing kubeconfig key in secret"}

type ErrorString struct {
	s string
}

func (e *ErrorString) Error() string {
	return e.s
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	controller := NewSecretController(clientset)

	stopCh := make(chan struct{})
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go controller.Run(stopCh)

	<-signalCh
	close(stopCh)
}

