package controllers

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyma-project/compass-manager/api/v1beta1"
	"github.com/kyma-project/compass-manager/controllers/mocks"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg              *rest.Config
	k8sClient        client.Client
	testEnv          *envtest.Environment
	cm               *CompassManagerReconciler
	mockConfigurator *mocks.Configurator
	mockRegistrator  *mocks.Registrator
	suiteCtx         context.Context
	cancelSuiteCtx   context.CancelFunc
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "hack", "crd"), filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = kyma.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	mockConfigurator = &mocks.Configurator{}
	mockRegistrator = &mocks.Registrator{}
	prepareMockFunctions(mockConfigurator, mockRegistrator)

	requeueTime := time.Second * 10

	cm = NewCompassManagerReconciler(k8sManager, log, mockConfigurator, mockRegistrator, requeueTime)
	k8sClient = k8sManager.GetClient()
	err = cm.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	Expect(createNamespace(kymaCustomResourceNamespace)).To(Succeed())

	go func() {
		defer GinkgoRecover()

		suiteCtx, cancelSuiteCtx = context.WithCancel(context.Background())

		err = k8sManager.Start(suiteCtx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
	cancelSuiteCtx()

	By("tearing down the test environment")
	err := (func() (err error) {
		// Need to sleep if the first stop fails due to a bug:
		// https://github.com/kubernetes-sigs/controller-runtime/issues/1571
		sleepTime := 1 * time.Millisecond
		for i := 0; i < 12; i++ { // Exponentially sleep up to ~4s
			if err = testEnv.Stop(); err == nil {
				return
			}
			sleepTime *= 2
			time.Sleep(sleepTime)
		}
		return
	})()
	Expect(err).NotTo(HaveOccurred())
})

func prepareMockFunctions(c *mocks.Configurator, r *mocks.Registrator) {
	compassLabelsAllGood := createCompassRuntimeLabels(map[string]string{ShootNameLabel: "all-good", GlobalAccountIDLabel: "globalAccount"})
	compassLabelsConfigureFails := createCompassRuntimeLabels(map[string]string{ShootNameLabel: "configure-fails", GlobalAccountIDLabel: "globalAccount"})
	compassLabelsRegistrationFails := createCompassRuntimeLabels(map[string]string{ShootNameLabel: "registration-fails", GlobalAccountIDLabel: "globalAccount"})
	compassLabelsEmptyKubeconfig := createCompassRuntimeLabels(map[string]string{ShootNameLabel: "empty-kubeconfig", GlobalAccountIDLabel: "globalAccount"})

	// Feature (refreshing token) is implemented but according to our discussions, it will be a part of another PR
	// compassLabelsRefreshToken := createCompassRuntimeLabels(map[string]string{ShootNameLabel: "refresh-token", GlobalAccountIDLabel: "globalAccount"})
	//refreshedToken := graphql.OneTimeTokenForRuntimeExt{
	//	OneTimeTokenForRuntime: graphql.OneTimeTokenForRuntime{},
	//	Raw:                    "rawToken",
	//	RawEncoded:             "rawEncodedToken",
	//}

	r.On("RegisterInCompass", compassLabelsAllGood).Return("id-all-good", nil)
	c.On("ConfigureCompassRuntimeAgent", "kubeconfig-data-all-good", "id-all-good").Return(nil)

	// The first call to ConfigureRuntimeAgent fails, but the second is successful
	r.On("RegisterInCompass", compassLabelsConfigureFails).Return("id-configure-fails", nil)
	c.On("ConfigureCompassRuntimeAgent", "kubeconfig-data-configure-fails", "id-configure-fails").Return(errors.New("error during configuration of Compass Runtime Agent CR")).Once()
	c.On("ConfigureCompassRuntimeAgent", "kubeconfig-data-configure-fails", "id-configure-fails").Return(nil).Once()

	// The first call to RegisterInCompass fails, but the second is successful.
	r.On("RegisterInCompass", compassLabelsRegistrationFails).Return("", errors.New("error during registration")).Once()
	r.On("RegisterInCompass", compassLabelsRegistrationFails).Return("registration-fails", nil).Once()
	c.On("ConfigureCompassRuntimeAgent", "kubeconfig-data-registration-fails", "registration-fails").Return(nil)

	r.On("RegisterInCompass", compassLabelsEmptyKubeconfig).Return("id-empty-kubeconfig", nil)
	c.On("ConfigureCompassRuntimeAgent", "kubeconfig-data-empty-kubeconfig", "id-empty-kubeconfig").Return(nil)

	// Feature (refreshing token) is implemented but according to our discussions, it will be a part of another PR
	// r.On("RegisterInCompass", compassLabelsRefreshToken).Return("id-refresh-token", nil).Once()
	//c.On("ConfigureCompassRuntimeAgent", "kubeconfig-data-refresh-token", "id-refresh-token").Return(nil).Once()
	//r.On("RefreshCompassToken", "id-refresh-token", "globalAccount").Return(refreshedToken, nil).Once()
}
