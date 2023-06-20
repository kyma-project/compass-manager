package controllers

import (
	"context"
	"time"

	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type testHelper struct {
	ctx                         context.Context
	kymaCustomResourceName      string
	kymaCustomResourceNamespace string
	clientTimeout               time.Duration
	clientInterval              time.Duration
}

const (
	kymaCustomResourceName       = "test-kyma-cr"
	kymaCustomResourceNamespace  = "kcp-system"
	kymaCustomResourceKind       = "Kyma"
	kymaCustomResourceAPIVersion = "operator.kyma-project.io/v1beta1"
)

var _ = Describe("Compass Manager controller", func() {

	kymaCustomResourceLabels := make(map[string]string)
	kymaCustomResourceLabels["operator.kyma-project.io/managed-by"] = "lifecycle-manager"

	h := testHelper{
		ctx:                         context.Background(),
		kymaCustomResourceName:      kymaCustomResourceName,
		kymaCustomResourceNamespace: kymaCustomResourceNamespace,
		clientTimeout:               time.Second * 30,
		clientInterval:              time.Second * 3,
	}

	Context("Secret with Kubeconfig is correctly created, and assigned to Kyma resource", func() {
		When("Runtime successfully registered, and Compass Runtime Agent's configuration created", func() {
			It("Should set compass-id label on Kyma CR", func() {

				By("Create secret with credentials")
				secret := createCredentialsSecret("all-good", kymaCustomResourceNamespace)
				Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

				By("Create Kyma Resource")
				kyma := createKymaResource("all-good")
				Expect(k8sClient.Create(h.ctx, &kyma)).To(Succeed())

				Eventually(func() bool {
					label, err := getKymaLabel(kyma.Name, "operator.kyma-project.io/compass-id", kymaCustomResourceNamespace)

					return err == nil && label != ""
				}, h.clientTimeout, h.clientInterval).Should(BeTrue())
			})
		})
		DescribeTable("Controller failed to register, or configure Runtime", func(kymaName string) {
			By("Create Kyma Resource")
			kyma := createKymaResource(kymaName)
			Expect(k8sClient.Create(h.ctx, &kyma)).To(Succeed())

			By("Create secret with credentials")
			secret := createCredentialsSecret(kyma.Name, kymaCustomResourceNamespace)
			Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

			Consistently(func() bool {
				label, err := getKymaLabel(kyma.Name, "operator.kyma-project.io/compass-id", kymaCustomResourceNamespace)

				return err == nil && label == ""
			}, h.clientTimeout, h.clientInterval).Should(BeTrue())
		},
			Entry("Runtime successfully registered, and Compass Runtime Agent's configuration creation failed", "configure-fails"),
			Entry("Runtime Registration Error", "registration-fails"),
		)
	})

	Context("When secret with Kubeconfig is not present on environment", func() {
		It("requeue the request if and succeeded when user add the secret", func() {

			By("Create Kyma Resource")
			kyma := createKymaResource("empty-kubeconfig")
			Expect(k8sClient.Create(h.ctx, &kyma)).To(Succeed())

			Consistently(func() bool {
				label, err := getKymaLabel(kyma.Name, "operator.kyma-project.io/compass-id", kymaCustomResourceNamespace)

				return err == nil && label == ""
			}, h.clientTimeout, h.clientInterval).Should(BeTrue())

			By("Create secret with credentials")
			secret := createCredentialsSecret(kyma.Name, kymaCustomResourceNamespace)
			Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

			Eventually(func() bool {
				label, err := getKymaLabel(kyma.Name, "operator.kyma-project.io/compass-id", kymaCustomResourceNamespace)

				return err == nil && label != ""
			}, h.clientTimeout, h.clientInterval).Should(BeTrue())
		})
	})
})

func (h *testHelper) shouldUpdateKyma(name, namespace string) {
	var obj kyma.Kyma
	key := types.NamespacedName{Name: name, Namespace: namespace}

	Eventually(func() bool {
		err := cm.Client.Get(context.Background(), key, &obj)
		if err != nil {
			return false
		}

		_, labelFound := obj.GetLabels()[CompassIDLabel]
		if !labelFound {
			return false
		}

		obj.Spec.Channel = "fast"

		err = cm.Client.Update(context.Background(), &obj)
		return err == nil
	}, h.clientTimeout, h.clientInterval).Should(BeTrue())
}

func (h *testHelper) shouldCreateSecret(name, namespace string, labels map[string]string, secretData map[string][]byte) {
	obj := corev1.Secret{
		TypeMeta:   metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Immutable:  nil,
		Data:       secretData,
		StringData: nil,
		Type:       "Opaque",
	}
	Expect(k8sClient.Create(context.Background(), &obj)).To(Succeed())
}

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
	kymaCustomResourceLabels["operator.kyma-project.io/managed-by"] = "lifecycle-manager"

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
			Modules: nil,
			Sync: kyma.Sync{
				Enabled:       true,
				Strategy:      "secret",
				Namespace:     kymaCustomResourceNamespace,
				NoModuleCopy:  false,
				ModuleCatalog: false,
			},
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
