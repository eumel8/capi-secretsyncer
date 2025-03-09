package main

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes"
)

func TestHandleSecretAdd(t *testing.T) {
	// Create fake clientset with tenant-kubeconfig Secret
	kubeconfigData := []byte("apiVersion: v1\nclusters:\n- cluster:\n    server: https://dummy\n  name: dummy\ncontexts:\n- context:\n    cluster: dummy\n    user: dummy\n  name: dummy\ncurrent-context: dummy\nkind: Config\npreferences: {}\nusers:\n- name: dummy\n  user:\n    token: dummy")

	client := fakeclientset.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{
			tenantKubeconfigSecretKey: kubeconfigData,
		},
	})

	fakeTenantClient := fakeclientset.NewSimpleClientset()

	controller := &SecretController{
		clientset: client,
		getTenantClientFunc: func(secret *corev1.Secret) (kubernetes.Interface, error) {
			return fakeTenantClient, nil
		},
	}

	// Create a source secret to sync
	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	controller.handleSecretAdd(srcSecret)

	// Validate secret was created in tenant cluster (simulated in fake client)
	createdSecret, err := fakeTenantClient.CoreV1().Secrets("default").Get(context.TODO(), "my-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Secret not created: %v", err)
	}

	if string(createdSecret.Data["key"]) != "value" {
		t.Errorf("Expected secret data 'value', got '%s'", string(createdSecret.Data["key"]))
	}
}

func TestHandleSecretAdd_InvalidKubeconfig(t *testing.T) {
	client := fakeclientset.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{
			tenantKubeconfigSecretKey: []byte("not-valid-kubeconfig"),
		},
	})

	controller := &SecretController{
		clientset: client,
		getTenantClientFunc: func(secret *corev1.Secret) (kubernetes.Interface, error) {
			return nil, errInvalidKubeconfig
		},
	}

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-secret",
			Namespace: "default",
		},
	}

	controller.handleSecretAdd(srcSecret)

	_, err := client.CoreV1().Secrets("default").Get(context.TODO(), "bad-secret", metav1.GetOptions{})
	if err == nil {
		t.Errorf("Expected no secret to be created, but found one")
	}
}

var errInvalidKubeconfig = &FakeError{"invalid kubeconfig"}

type FakeError struct {
	s string
}

func (e *FakeError) Error() string {
	return e.s
}

