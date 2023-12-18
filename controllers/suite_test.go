package controllers

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyma-project/compass-manager/api/v1beta1"
	"github.com/kyma-project/compass-manager/controllers/metrics"
	"github.com/kyma-project/compass-manager/controllers/mocks"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2" //nolint:revive
	. "github.com/onsi/gomega"    //nolint:revive
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
	cfg              *rest.Config              //nolint:gochecknoglobals
	k8sClient        client.Client             //nolint:gochecknoglobals
	testEnv          *envtest.Environment      //nolint:gochecknoglobals
	cm               *CompassManagerReconciler //nolint:gochecknoglobals
	mockConfigurator *mocks.Configurator       //nolint:gochecknoglobals
	mockRegistrator  *mocks.Registrator        //nolint:gochecknoglobals
	suiteCtx         context.Context           //nolint:gochecknoglobals
	cancelSuiteCtx   context.CancelFunc        //nolint:gochecknoglobals
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

	requeueTime := time.Second * 5
	metrics := metrics.NewMetrics()

	cm = NewCompassManagerReconciler(
		k8sManager,
		log,
		mockConfigurator,
		mockRegistrator,
		requeueTime,
		true,
		metrics,
	)
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
	// It handles `compass-runtime-id-for-migration`
	compassLabelsRegistered := createCompassRuntimeLabels(map[string]string{LabelShootName: "preregistered", LabelGlobalAccountID: "globalAccount"})
	r.On("RegisterInCompass", compassLabelsRegistered).Return("id-preregistered-incorrect", nil)
	// succeeding test case
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-preregistered"), "preregistered-id", "globalAccount").Return(nil)
	// failing test case
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-preregistered"), "preregistered-id", "globalAccount").Return(errors.New("this shouldn't be called"))

	compassLabelsAllGood := createCompassRuntimeLabels(map[string]string{LabelShootName: "all-good", LabelGlobalAccountID: "globalAccount"})
	r.On("RegisterInCompass", compassLabelsAllGood).Return("id-all-good", nil)
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-all-good"), "id-all-good", "globalAccount").Return(nil)

	compassLabelsConfigureFails := createCompassRuntimeLabels(map[string]string{LabelShootName: "configure-fails", LabelGlobalAccountID: "globalAccount"})
	// The first call to ConfigureRuntimeAgent fails, but the second is successful
	r.On("RegisterInCompass", compassLabelsConfigureFails).Return("id-configure-fails", nil)
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-configure-fails"), "id-configure-fails", "globalAccount").Return(errors.New("error during configuration of Compass Runtime Agent CR")).Once()
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-configure-fails"), "id-configure-fails", "globalAccount").Return(nil).Once()

	compassLabelsRegistrationFails := createCompassRuntimeLabels(map[string]string{LabelShootName: "registration-fails", LabelGlobalAccountID: "globalAccount"})
	// The first call to RegisterInCompass fails, but the second is successful.
	r.On("RegisterInCompass", compassLabelsRegistrationFails).Return("", errors.New("error during registration")).Once()
	r.On("RegisterInCompass", compassLabelsRegistrationFails).Return("registration-fails", nil).Once()
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-registration-fails"), "registration-fails", "globalAccount").Return(nil)

	compassLabelsEmptyKubeconfig := createCompassRuntimeLabels(map[string]string{LabelShootName: "empty-kubeconfig", LabelGlobalAccountID: "globalAccount"})
	r.On("RegisterInCompass", compassLabelsEmptyKubeconfig).Return("id-empty-kubeconfig", nil)
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-empty-kubeconfig"), "id-empty-kubeconfig", "globalAccount").Return(nil)

	compassLabelsDeregistration := createCompassRuntimeLabels(map[string]string{LabelShootName: "unregister-runtime", LabelGlobalAccountID: "globalAccount"})
	r.On("RegisterInCompass", compassLabelsDeregistration).Return("id-unregister-runtime", nil)
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-unregister-runtime"), "id-unregister-runtime", "globalAccount").Return(nil)
	r.On("DeregisterFromCompass", "id-unregister-runtime", "globalAccount").Return(nil)

	compassLabelsDeregistrationFails := createCompassRuntimeLabels(map[string]string{LabelShootName: "unregister-runtime-fails", LabelGlobalAccountID: "globalAccount"})
	r.On("RegisterInCompass", compassLabelsDeregistrationFails).Return("id-unregister-runtime-fails", nil)
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-unregister-runtime-fails"), "id-unregister-runtime-fails", "globalAccount").Return(nil)
	r.On("DeregisterFromCompass", "id-unregister-runtime-fails", "globalAccount").Return(errors.New("error during unregistration of the runtime")).Once()
	r.On("DeregisterFromCompass", "id-unregister-runtime-fails", "globalAccount").Return(nil).Once()

	compassLabelsRefreshToken := createCompassRuntimeLabels(map[string]string{LabelShootName: "refresh-token", LabelGlobalAccountID: "globalAccount"})
	r.On("RegisterInCompass", compassLabelsRefreshToken).Return("id-refresh-token", nil).Once()
	c.On("ConfigureCompassRuntimeAgent", []byte("kubeconfig-data-refresh-token"), "id-refresh-token", "globalAccount").Return(nil).Twice()
}
