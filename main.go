package main

import (
        "context"
        //"fmt"
        "os"

        corev1 "k8s.io/api/core/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/client-go/informers"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/rest"
        "k8s.io/client-go/tools/cache"
        "k8s.io/client-go/tools/clientcmd"
        "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
        watchedSecretLabel        = "sync-to-tenant"
        tenantKubeconfigSecretKey = "kubeconfig"
)

type SecretController struct {
        clientset *kubernetes.Clientset
}

func NewSecretController(clientset *kubernetes.Clientset) *SecretController {
        return &SecretController{clientset: clientset}
}

func (c *SecretController) handleSecretAdd(obj interface{}) {
        secret, ok := obj.(*corev1.Secret)
        if !ok {
                return
        }

        log := zap.New()
        log.Info("Secret added", "namespace", secret.Namespace, "name", secret.Name)

        // Fetch the tenant kubeconfig Secret
        kubeconfigSecret, err := c.clientset.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), "tenant-kubeconfig", metav1.GetOptions{})
        if err != nil {
                log.Error(err, "Failed to get tenant kubeconfig Secret")
                return
        }

        kubeconfigData, ok := kubeconfigSecret.Data[tenantKubeconfigSecretKey]
        if !ok {
                log.Error(nil, "Kubeconfig key missing in tenant kubeconfig Secret")
                return
        }

        tenantConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
        if err != nil {
                log.Error(err, "Failed to parse tenant kubeconfig")
                return
        }

        tenantClient, err := kubernetes.NewForConfig(tenantConfig)
        if err != nil {
                log.Error(err, "Failed to create tenant client")
                return
        }

        // Deploy the Secret to the tenant cluster
        tenantSecret := &corev1.Secret{
                ObjectMeta: metav1.ObjectMeta{
                        Namespace: secret.Namespace,
                        Name:      secret.Name,
                },
                Data: secret.Data,
        }

        _, err = tenantClient.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), tenantSecret, metav1.CreateOptions{})
        if err != nil {
                log.Error(err, "Failed to create Secret in tenant cluster")
                return
        }

        log.Info("Successfully synced Secret to tenant cluster")
}

func main() {
        logger := zap.New()

        cfg, err := rest.InClusterConfig()
        if err != nil {
                logger.Error(err, "Failed to get in-cluster config")
                os.Exit(1)
        }

        clientset, err := kubernetes.NewForConfig(cfg)
        if err != nil {
                logger.Error(err, "Failed to create Kubernetes client")
                os.Exit(1)
        }

        factory := informers.NewSharedInformerFactory(clientset, 0)
        secretInformer := factory.Core().V1().Secrets().Informer()

        controller := NewSecretController(clientset)
        secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
                AddFunc: controller.handleSecretAdd,
        })

        stopCh := make(chan struct{})
        logger.Info("Starting the operator")
        factory.Start(stopCh)
        cache.WaitForCacheSync(stopCh, secretInformer.HasSynced)
        <-stopCh
}
