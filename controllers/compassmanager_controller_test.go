package controllers

import (
	"context"
	"time"

	"github.com/kyma-project/compass-manager/api/v1beta1"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	It("handles `compass-runtime-id-for-migration`", func() {
		const kymaName = "preregistered"
		const preRegisteredID = "preregistered-id"

		secret := createCredentialsSecret(kymaName, kymaCustomResourceNamespace)
		Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

		By("Create Kyma Resource")
		kymaCR := createKymaResource(kymaName)
		kymaCR.Annotations["compass-runtime-id-for-migration"] = preRegisteredID
		Expect(k8sClient.Create(context.Background(), &kymaCR)).To(Succeed())

		Eventually(func() string {
			label, err := getCompassMappingLabel(kymaCR.Name, LabelComppassID, kymaCustomResourceNamespace)
			if err != nil {
				return err.Error()
			}
			return label
		}, clientTimeout, clientInterval).Should(Equal(preRegisteredID))
	})

	Context("Secret with Kubeconfig is correctly created, and assigned to Kyma resource", func() {
		DescribeTable("Register Runtime in the Director, and configure Compass Runtime Agent", func(kymaName string) {
			By("Create secret with credentials")
			secret := createCredentialsSecret(kymaName, kymaCustomResourceNamespace)
			Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

			By("Create Kyma Resource")
			kymaCR := createKymaResource(kymaName)
			Expect(k8sClient.Create(context.Background(), &kymaCR)).To(Succeed())

			Eventually(func() bool {
				label, err := getCompassMappingLabel(kymaCR.Name, LabelComppassID, kymaCustomResourceNamespace)

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
			kymaCR := createKymaResource("empty-kubeconfig")
			Expect(k8sClient.Create(context.Background(), &kymaCR)).To(Succeed())

			Consistently(func() bool {
				label, err := getCompassMappingLabel(kymaCR.Name, LabelComppassID, kymaCustomResourceNamespace)

				return errors.IsNotFound(err) && label == ""
			}, clientTimeout, clientInterval).Should(BeTrue())

			By("Create secret with credentials")
			secret := createCredentialsSecret(kymaCR.Name, kymaCustomResourceNamespace)
			Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())

			Eventually(func() bool {
				label, err := getCompassMappingLabel(kymaCR.Name, LabelComppassID, kymaCustomResourceNamespace)

				return err == nil && label != ""
			}, clientTimeout, clientInterval).Should(BeTrue())
		})
	})

	// Feature (refreshing token) is implemented but according to our discussions, it will be a part of another PR

	// Context("After successful runtime registration when user re-enable Application Connector module", func() {
	//	DescribeTable("the one-time token for Compass Runtime Agent should be refreshed", func(kymaName string) {
	//		By("Create secret with credentials")
	//		secret := createCredentialsSecret(kymaName, kymaCustomResourceNamespace)
	//		Expect(k8sClient.Create(context.Background(), &secret)).To(Succeed())
	//
	//		By("Create Kyma Resource")
	//		kymaCR := createKymaResource(kymaName)
	//		Expect(k8sClient.Create(context.Background(), &kymaCR)).To(Succeed())
	//
	//		Eventually(func() bool {
	//			label, err := getCompassMappingLabel(kymaCR.Name, ComppassIDLabel, kymaCustomResourceNamespace)
	//
	//			return err == nil && label != ""
	//		}, clientTimeout, clientInterval).Should(BeTrue())
	//
	//		By("Disable the Application Connector module")
	//		modifiedKyma, err := modifyKymaModules(kymaCR.Name, kymaCustomResourceNamespace, nil)
	//		Expect(err).NotTo(HaveOccurred())
	//		Expect(k8sClient.Update(context.Background(), modifiedKyma)).To(Succeed())
	//
	//		By("Re-enable the Application Connector module")
	//		kymaModules := make([]kyma.Module, 2)
	//		kymaModules[0].Name = ApplicationConnectorModuleName
	//		kymaModules[1].Name = "test-module"
	//		modifiedKyma, err = modifyKymaModules(kymaCR.Name, kymaCustomResourceNamespace, kymaModules)
	//		Expect(err).NotTo(HaveOccurred())
	//		Expect(k8sClient.Update(context.Background(), modifiedKyma)).To(Succeed())
	//	},
	//		Entry("Token successfully refreshed", "refresh-token"),
	//	)
	// })
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
	kymaCustomResourceLabels[LabelGlobalAccountID] = "globalAccount"
	kymaCustomResourceLabels[LabelShootName] = name
	kymaCustomResourceLabels[LabelKymaName] = name

	kymaModules := make([]kyma.Module, 1)
	kymaModules[0].Name = ApplicationConnectorModuleName

	return kyma.Kyma{
		TypeMeta: metav1.TypeMeta{
			Kind:       kymaCustomResourceKind,
			APIVersion: kymaCustomResourceAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   kymaCustomResourceNamespace,
			Labels:      kymaCustomResourceLabels,
			Annotations: make(map[string]string),
		},
		Spec: kyma.KymaSpec{
			Channel: "regular",
			Modules: kymaModules,
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

func getCompassMappingLabel(kymaName, labelName, namespace string) (string, error) {
	var obj v1beta1.CompassManagerMapping
	key := types.NamespacedName{Name: kymaName, Namespace: namespace}

	err := cm.Client.Get(context.Background(), key, &obj)
	if err != nil {
		return "", err
	}

	labels := obj.GetLabels()
	return labels[labelName], nil
}

// Feature (refreshing token) is implemented but according to our discussions, it will be a part of another PR

// func modifyKymaModules(kymaName, kymaNamespace string, kymaModules []kyma.Module) (*kyma.Kyma, error) {
//	var obj kyma.Kyma
//	key := types.NamespacedName{Name: kymaName, Namespace: kymaNamespace}
//
//	err := cm.Client.Get(context.Background(), key, &obj)
//	if err != nil {
//		return &kyma.Kyma{}, err
//	}
//
//	obj.Spec.Modules = kymaModules
//
//	return &obj, nil
//  }
