package controllers

import (
	"context"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (cm *CompassManagerSuite) TestController() {

	const (
		kymaCustomResourceName       = "test-kyma-cr"
		kymaCustomResourceNamespace  = "kcp-system"
		kymaCustomResourceKind       = "Kyma"
		kymaCustomResourceAPIVersion = "operator.kyma-project.io/v1beta1"
	)

	var kymaCustomResourceLabels map[string]string
	kymaCustomResourceLabels = make(map[string]string)
	kymaCustomResourceLabels["operator.kyma-project.io/managed-by"] = "lifecycle-manager"

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

	cm.T().Logf("Creating namespace: %s", kymaCustomResourceNamespace)
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: kymaCustomResourceNamespace,
		},
	}
	err := cm.k8sClient.Create(context.Background(), &namespace)
	cm.Require().NoError(err)
	cm.T().Logf("Namespace created: %s", kymaCustomResourceNamespace)

	cm.Run("Should enter the reconciliation loop, invoke registration of runtime in Compass and invoke creation of Compass Runtime Agent CR on cluster", func() {

		//given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "all-good"

		//when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

	})

	cm.Run("Should enter the reconciliation loop, invoke registration of runtime in Compass and return error during creation of Compass Runtime Agent CR on cluster", func() {

		//given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "pass"

		//when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

	})

	cm.Run("Should enter the reconciliation loop, return error during registration of runtime in Compass and quit", func() {

		//given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "fail-only"

		//when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

	})

	cm.Run("Should do not enter reconciliation loop if Kubeconfig was not provided in Kyma CR", func() {
		//given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "empty-kubeconfig"
		testSuiteKyma.Spec.Sync.Strategy = ""

		//when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)
	})
	cm.Run("Should do not enter reconciliation loop if an insignificant field in Kyma CR has been changed", func() {
		//given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "insignificant-field"

		//when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

		cm.shouldUpdateKyma(testSuiteKyma.Name, testSuiteKyma.Namespace)
	})

	//then
	cm.mockRegister.AssertExpectations(cm.T())
}

func (cm *CompassManagerSuite) shouldCreateKyma(kymaName string, obj kyma.Kyma) {
	//act
	cm.T().Logf("Creating cr: %s", kymaName)
	err := cm.k8sClient.Create(context.Background(), &obj)
	cm.Require().NoError(err)
	cm.T().Logf("Cr created: %s", kymaName)
}

func (cm *CompassManagerSuite) shouldUpdateKyma(name, namespace string) {
	//act
	var obj kyma.Kyma
	key := types.NamespacedName{Name: name, Namespace: namespace}

	err := cm.k8sClient.Get(context.Background(), key, &obj)
	cm.Require().NoError(err)

	obj.Spec.Channel = "fast"
	err = cm.k8sClient.Update(context.Background(), &obj)
	cm.Require().NoError(err)
}
