package controllers

import (
	"context"
	"time"

	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	kymaCustomResourceNamespace  = "kcp-system"
	kymaCustomResourceKind       = "Kyma"
	kymaCustomResourceAPIVersion = "operator.kyma-project.io/v1beta2"
	clientTimeout                = time.Second * 30
	clientInterval               = time.Second * 3
)

var _ = Describe("Compass Manager controller", func() {

	kymaCustomResourceLabels := make(map[string]string)
	kymaCustomResourceLabels["operator.kyma-project.io/managed-by"] = "lifecycle-manager"

	Context("Secret with Kubeconfig is correctly created, and assigned to Kyma resource", func() {
		DescribeTable("Register Runtime in the Director, and configure Compass Runtime Agent", func(kymaName string) {
			By("Create secret with credentials")
			secret := createCredentialsSecret(kymaName, kymaCustomResourceNamespace)
			Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

			By("Create Kyma Resource")
			kyma := createKymaResource(kymaName)
			Expect(k8sClient.Create(context.Background(), &kyma)).To(Succeed())

			Eventually(func() bool {
				label, err := getKymaLabel(kyma.Name, "operator.kyma-project.io/compass-id", kymaCustomResourceNamespace)

				return err == nil && label != ""
			}, clientTimeout, clientInterval).Should(BeTrue())
		},
			Entry("Runtime successfully registered, and Compass Runtime Agent's configuration created", "all-good"),
			Entry("The first attempt to register Runtime failed, and retry succeeded", "registration-fails"),
			Entry("Runtime successfully registered, the first attempt to configure Compass Runtime Agent failed, and retry succeeded", "configure-fails"),
		)
	})

	Context("When secret with Kubeconfig is not present on environment", func() {
		It("requeue the request if and succeeded when user add the secret", func() {

			By("Create Kyma Resource")
			kyma := createKymaResource("empty-kubeconfig")
			Expect(k8sClient.Create(context.Background(), &kyma)).To(Succeed())

			Consistently(func() bool {
				label, err := getKymaLabel(kyma.Name, "operator.kyma-project.io/compass-id", kymaCustomResourceNamespace)

				return err == nil && label == ""
			}, clientTimeout, clientInterval).Should(BeTrue())

			By("Create secret with credentials")
			secret := createCredentialsSecret(kyma.Name, kymaCustomResourceNamespace)
			Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

			Eventually(func() bool {
				label, err := getKymaLabel(kyma.Name, "operator.kyma-project.io/compass-id", kymaCustomResourceNamespace)

				return err == nil && label != ""
			}, clientTimeout, clientInterval).Should(BeTrue())
		})
	})
})

func createNamespace(name string) error {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return k8sClient.Create(context.Background(), &namespace)
}

func createKymaResource(name string) kyma.Kyma {
	kymaCustomResourceLabels := make(map[string]string)
	kymaCustomResourceLabels[GlobalAccountIDLabel] = "globalAccount"
	kymaCustomResourceLabels[ShootNameLabel] = name

	return kyma.Kyma{
		TypeMeta: metav1.TypeMeta{
			Kind:       kymaCustomResourceKind,
			APIVersion: kymaCustomResourceAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kymaCustomResourceNamespace,
			Labels:    kymaCustomResourceLabels,
		},
		Spec: kyma.KymaSpec{
			Channel: "regular",
		},
	}
}

func createCredentialsSecret(kymaName, namespace string) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: namespace,
			Labels:    map[string]string{"operator.kyma-project.io/kyma-name": kymaName},
		},
		Immutable:  nil,
		Data:       map[string][]byte{KubeconfigKey: []byte("kubeconfig-data-" + kymaName)},
		StringData: nil,
		Type:       "Opaque",
	}
}

func getKymaLabel(kymaName, labelName, namespace string) (string, error) {
	var obj kyma.Kyma
	key := types.NamespacedName{Name: kymaName, Namespace: namespace}

	err := cm.Client.Get(context.Background(), key, &obj)
	if err != nil {
		return "", err
	}

	labels := obj.GetLabels()
	return labels[labelName], nil
}
