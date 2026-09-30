package controllers

import (
	"context"
	"testing"

	"github.com/kyma-project/compass-manager/api/v1beta1"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileReturnsErrorWhenGlobalAccountLabelMissing(t *testing.T) {
	t.Parallel()

	testScheme := runtime.NewScheme()
	if err := corev1.AddToScheme(testScheme); err != nil {
		t.Fatal(err)
	}
	if err := kyma.AddToScheme(testScheme); err != nil {
		t.Fatal(err)
	}
	if err := v1beta1.AddToScheme(testScheme); err != nil {
		t.Fatal(err)
	}

	name := types.NamespacedName{Name: "missing-global-account", Namespace: "kcp-system"}
	kymaCR := &kyma.Kyma{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    map[string]string{LabelKymaName: name.Name},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    map[string]string{LabelKymaName: name.Name},
		},
		Data: map[string][]byte{KubeconfigKey: []byte("kubeconfig")},
	}
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(kymaCR, secret).Build()
	logger := logrus.New()
	reconciler := &CompassManagerReconciler{
		Log:     logger,
		cluster: NewControlPlaneInterface(client, logger, false),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: name})
	if err == nil {
		t.Fatal("expected missing global account label to return an error")
	}
}
