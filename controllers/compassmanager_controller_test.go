package controllers

import (
	"context"
	"fmt"
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

var _ = Describe("Compass Manager controller", func() {

	const (
		kymaCustomResourceName       = "test-kyma-cr"
		kymaCustomResourceNamespace  = "kcp-system"
		kymaCustomResourceKind       = "Kyma"
		kymaCustomResourceAPIVersion = "operator.kyma-project.io/v1beta1"
	)

	kymaCustomResourceLabels := make(map[string]string)
	kymaCustomResourceLabels["operator.kyma-project.io/managed-by"] = "lifecycle-manager"
	var kymaResource *kyma.Kyma

	BeforeEach(func() {
		kymaResource = &kyma.Kyma{
			TypeMeta: metav1.TypeMeta{
				Kind:       kymaCustomResourceKind,
				APIVersion: kymaCustomResourceAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      kymaCustomResourceName,
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
	})

	h := testHelper{
		ctx:                         context.Background(),
		kymaCustomResourceName:      kymaCustomResourceName,
		kymaCustomResourceNamespace: kymaCustomResourceNamespace,
		clientTimeout:               time.Second * 30,
		clientInterval:              time.Second * 3,
	}

	Context("When secret with Kubeconfig is correctly created and present on environment, and reconciliation was triggered", func() {
		When("successfully invoke registration of runtime in Compass and invoke creation of Compass Runtime Agent CR on cluster", func() {
			It("label with compass-id must be present on Kyma CR", func() {
				h.createNamespace()
				// given
				testSuiteKyma := kymaResource
				testSuiteKyma.Name = "all-good"
				kymaSecretLabels := make(map[string]string)
				kymaSecretLabels["operator.kyma-project.io/kyma-name"] = testSuiteKyma.Name
				h.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace, kymaSecretLabels)

				//when
				h.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

				//then
				h.shouldCheckCompassLabel(testSuiteKyma.Name, testSuiteKyma.Namespace, false)

			})
		})
		When("successfully invoke registration of runtime in Compass and return error during creation of Compass Runtime Agent CR on cluster", func() {
			It("label with-compass must not be present on Kyma CR", func() {
				// given
				testSuiteKyma := kymaResource
				testSuiteKyma.Name = "configure-fails"
				kymaSecretLabels := make(map[string]string)
				kymaSecretLabels["operator.kyma-project.io/kyma-name"] = testSuiteKyma.Name
				h.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace, kymaSecretLabels)

				// when
				h.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

				// then
				h.shouldCheckCompassLabel(testSuiteKyma.Name, testSuiteKyma.Namespace, true)
			})
		})
		When("return error during invoking the registration of runtime in Compass", func() {
			It("label with-compass must not be present on Kyma CR", func() {
				// given
				testSuiteKyma := kymaResource
				testSuiteKyma.Name = "registration-fails"
				kymaSecretLabels := make(map[string]string)
				kymaSecretLabels["operator.kyma-project.io/kyma-name"] = testSuiteKyma.Name
				h.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace, kymaSecretLabels)

				// when
				h.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

				// then
				h.shouldCheckCompassLabel(testSuiteKyma.Name, testSuiteKyma.Namespace, true)

			})
		})
	})

	Context("When secret with Kubeconfig is not present on environment", func() {
		It("requeue the request if and succeeded when user add the secret", func() {
			// given
			testSuiteKyma := kymaResource
			testSuiteKyma.Name = "empty-kubeconfig"

			// when
			h.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)
			kymaSecretLabels := make(map[string]string)
			kymaSecretLabels["operator.kyma-project.io/kyma-name"] = testSuiteKyma.Name
			h.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace, kymaSecretLabels)

			// then
			h.shouldCheckCompassLabel(testSuiteKyma.Name, testSuiteKyma.Namespace, false)

		})
	})
	Context("When an insignificant field in Kyma CR has been changed and label is present on Kyma CR should not enter reconcilation loop", func() {
		It("do not enter reconciliation loop", func() {
			// given
			testSuiteKyma := kymaResource
			testSuiteKyma.Name = "insignificant-field"
			kymaSecretLabels := make(map[string]string)
			kymaSecretLabels["operator.kyma-project.io/kyma-name"] = testSuiteKyma.Name
			h.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace, kymaSecretLabels)
			h.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

			// when
			h.shouldUpdateKyma(testSuiteKyma.Name, testSuiteKyma.Namespace)

		})
	})
})

func (h *testHelper) shouldCreateKyma(kymaCRName string, obj *kyma.Kyma) {
	By(fmt.Sprintf("Creating crd: %s", kymaCRName))
	Expect(k8sClient.Create(h.ctx, obj)).To(Succeed())
	By(fmt.Sprintf("Crd created: %s", kymaCRName))
}

func (h *testHelper) shouldUpdateKyma(name, namespace string) {
	var obj kyma.Kyma
	key := types.NamespacedName{Name: name, Namespace: namespace}

	Eventually(func() bool {
		err := cm.Client.Get(context.Background(), key, &obj)
		if err != nil {
			return false
		}

		obj.Spec.Channel = "fast"

		err = cm.Client.Update(context.Background(), &obj)
		if err != nil {
			return false
		}
		return true
	}, h.clientTimeout, h.clientInterval).Should(BeTrue())
}

func (h *testHelper) shouldCreateSecret(name, namespace string, labels map[string]string) {
	obj := corev1.Secret{
		TypeMeta:   metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Immutable:  nil,
		Data:       nil,
		StringData: nil,
		Type:       "Opaque",
	}
	Expect(k8sClient.Create(context.Background(), &obj)).To(Succeed())
}

func (h *testHelper) shouldCheckCompassLabel(name, namespace string, shouldBeMissing bool) {
	var obj kyma.Kyma
	key := types.NamespacedName{Name: name, Namespace: namespace}

	Eventually(func() bool {
		err := cm.Client.Get(context.Background(), key, &obj)
		if err != nil {
			return false
		}

		labels := obj.GetLabels()
		if shouldBeMissing {
			_, exists := labels["operator.kyma-project.io/compass-id"]
			return !exists
		}
		_, exists := labels["operator.kyma-project.io/compass-id"]
		return exists

	}, h.clientTimeout, h.clientInterval).ShouldNot(BeFalse())
}

func (h *testHelper) createNamespace() {
	By(fmt.Sprintf("Creating namespace: %s", h.kymaCustomResourceNamespace))
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.kymaCustomResourceNamespace,
		},
	}
	err := k8sClient.Create(h.ctx, &namespace)
	if err != nil {
		By(fmt.Sprintf("Cannot create namespace, aborting"))
		return
	}
	By(fmt.Sprintf("Namespace created: %s", h.kymaCustomResourceNamespace))
}
