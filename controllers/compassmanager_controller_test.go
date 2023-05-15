package controllers

import (
	"context"
	"fmt"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testHelper struct {
	ctx                         context.Context
	kymaCustomResourceName      string
	kymaCustomResourceNamespace string
}

// in the future, change the approach to checking the state of Compass Manager Custom Resource instead of checking reconciliation status
var _ = Describe("Compass Manager controller", func() {

	const (
		kymaCustomResourceName       = "test-kyma-cr"
		kymaCustomResourceNamespace  = "kcp-system"
		kymaCustomResourceKind       = "Kyma"
		kymaCustomResourceAPIVersion = "operator.kyma-project.io/v1beta1"
	)

	var kymaCustomResourceLabels map[string]string
	kymaCustomResourceLabels = make(map[string]string)
	kymaCustomResourceLabels["operator.kyma-project.io/managed-by"] = "lifecycle-manager"

	h := testHelper{
		ctx:                         context.Background(),
		kymaCustomResourceName:      kymaCustomResourceName,
		kymaCustomResourceNamespace: kymaCustomResourceNamespace,
	}

	//Context("When user provides empty kubeconfig field in Kyma Custom Resource", func() {
	//	It("The controller should skip reconciliation process")
	//})
	//Context("When user doesn't change kubeconfig field in Kyma Custom Resource", func() {
	//	It("The controller should skip reconciliation process")
	//})
	//Context("When user provides kubeconfig to previously empty field in Kyma Custom Resource", func() {
	//	It("The controller should enter the reconciliation process")
	//})
	//Context("When user change insignificant field in Kyma Custom Resource", func() {
	//	It("The controller should skip reconciliation process")
	//})
	Context("When user provides kubeconfig in Kyma Custom Resource", func() {

		var kymaResource = kyma.Kyma{
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
			Status: kyma.KymaStatus{},
		}
		It("The controller should enter the reconciliation process", func() {
			h.createNamespace()
			shouldCreateKyma(h, kymaCustomResourceName, kymaResource)
		})

	})
})

func shouldCreateKyma(h testHelper, kymaName string, obj kyma.Kyma) {
	//act
	h.createKymaCustomResource(kymaName, obj)
}

func (h *testHelper) createKymaCustomResource(kymaCRName string, obj kyma.Kyma) {
	By(fmt.Sprintf("Creating crd: %s", kymaCRName))
	Expect(k8sClient.Create(h.ctx, &obj)).To(Succeed())
	By(fmt.Sprintf("Crd created: %s", kymaCRName))
}

func (h *testHelper) createNamespace() {
	By(fmt.Sprintf("Creating namespace: %s", h.kymaCustomResourceNamespace))
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.kymaCustomResourceNamespace,
		},
	}
	Expect(k8sClient.Create(h.ctx, &namespace)).To(Succeed())
	By(fmt.Sprintf("Namespace created: %s", h.kymaCustomResourceNamespace))
}
