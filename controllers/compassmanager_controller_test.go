package controllers

import (
	"context"
	"time"

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

	kymaCustomResourceLabels := make(map[string]string)
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
	err := cm.compassManager.Client.Create(context.Background(), &namespace)
	cm.Require().NoError(err)
	cm.T().Logf("Namespace created: %s", kymaCustomResourceNamespace)

	cm.Run("Should enter the reconciliation loop, invoke registration of runtime in Compass and invoke creation of Compass Runtime Agent CR on cluster", func() {
		// given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "all-good"
		cm.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace)

		// when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

		// then
		time.Sleep(1 * time.Second)
		cm.shouldCheckLabels(testSuiteKyma.Name, testSuiteKyma.Namespace, false)
	})

	cm.Run("Should enter the reconciliation loop, invoke registration of runtime in Compass and return error during creation of Compass Runtime Agent CR on cluster", func() {
		// given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "configure-fails"
		cm.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace)

		// when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

		// then
		time.Sleep(1 * time.Second)
		cm.shouldCheckLabels(testSuiteKyma.Name, testSuiteKyma.Namespace, true)
	})

	cm.Run("Should enter the reconciliation loop, return error during registration of runtime in Compass", func() {
		// given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "registration-fails"
		cm.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace)

		// when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

		// then
		time.Sleep(1 * time.Second)
		cm.shouldCheckLabels(testSuiteKyma.Name, testSuiteKyma.Namespace, true)
	})

	cm.Run("Should enter reconciliation loop, requeue the request if Kubeconfig was not found on cluster and succeeded when user add the secret", func() {
		// given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "empty-kubeconfig"

		// when
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)
		time.Sleep(3 * time.Second)
		cm.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace)

		// then
		time.Sleep(10 * time.Second)
		cm.shouldCheckLabels(testSuiteKyma.Name, testSuiteKyma.Namespace, false)
	})

	cm.Run("Should do not enter reconciliation loop if an insignificant field in Kyma CR has been changed and label is present on Kyma CR", func() {
		// given
		testSuiteKyma := kymaResource
		testSuiteKyma.Name = "insignificant-field"
		cm.shouldCreateSecret("kubeconfig-"+testSuiteKyma.Name, testSuiteKyma.Namespace)
		cm.shouldCreateKyma(testSuiteKyma.Name, testSuiteKyma)

		// when
		time.Sleep(time.Second * 3)
		cm.shouldUpdateKyma(testSuiteKyma.Name, testSuiteKyma.Namespace)
	})

	// then
	cm.mockRegister.AssertExpectations(cm.T())
}

func (cm *CompassManagerSuite) shouldCreateKyma(kymaName string, obj kyma.Kyma) {
	// act
	cm.T().Logf("Creating cr: %s", kymaName)
	err := cm.compassManager.Client.Create(context.Background(), &obj)
	cm.Require().NoError(err)
	cm.T().Logf("Cr created: %s", kymaName)
}

func (cm *CompassManagerSuite) shouldUpdateKyma(name, namespace string) {
	// act
	var obj kyma.Kyma
	key := types.NamespacedName{Name: name, Namespace: namespace}

	err := cm.compassManager.Client.Get(context.Background(), key, &obj)
	cm.Require().NoError(err)

	obj.Spec.Channel = "fast"
	err = cm.compassManager.Client.Update(context.Background(), &obj)
	cm.Require().NoError(err)
}

func (cm *CompassManagerSuite) shouldCreateSecret(name, namespace string) {
	obj := corev1.Secret{
		TypeMeta:   metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Immutable:  nil,
		Data:       nil,
		StringData: nil,
		Type:       "Opaque",
	}
	err := cm.compassManager.Client.Create(context.Background(), &obj)
	cm.Require().NoError(err)
}

func (cm *CompassManagerSuite) shouldCheckLabels(name, namespace string, shouldBeMissing bool) {
	var obj kyma.Kyma
	key := types.NamespacedName{Name: name, Namespace: namespace}

	err := cm.compassManager.Client.Get(context.Background(), key, &obj)
	cm.Require().NoError(err)

	labels := obj.GetLabels()

	if shouldBeMissing {
		if _, ok := labels["operator.kyma-project.io/compass-id"]; ok {
			cm.FailNow("Kyma CR contain the compass-id label")
		}
	} else {
		if _, ok := labels["operator.kyma-project.io/compass-id"]; !ok {
			cm.FailNow("Kyma CR does not contain the compass-id label")
		}
	}
}
