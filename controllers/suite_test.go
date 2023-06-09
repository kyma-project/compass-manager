package controllers

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyma-project/compass-manager/controllers/mocks"

	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var cm *CompassManagerReconciler
var mockRegister *mocks.Registrator

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "hack", "crd")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	requeueTime = time.Second * 10
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = kyma.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	mockRegister = &mocks.Registrator{}
	prepareMockFunctions(mockRegister)

	cm = NewCompassManagerReconciler(k8sManager, log, mockRegister)
	k8sClient = k8sManager.GetClient()
	err = cm.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	Expect(createNamespace(kymaCustomResourceNamespace)).To(Succeed())

	go func() {
		defer GinkgoRecover()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
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

func prepareMockFunctions(r *mocks.Registrator) {
	r.On("Register", "all-good").Return("id-all-good", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-data-all-good", "id-all-good").Return(nil)

	// The first call to ConfigureRuntimeAgent fails, but the second is successful
	r.On("Register", "configure-fails").Return("id-configure-fails", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-data-configure-fails", "id-configure-fails").Return(errors.New("error during configuration of Compass Runtime Agent CR")).Once()
	r.On("ConfigureRuntimeAgent", "kubeconfig-data-configure-fails", "id-configure-fails").Return(nil).Once()

	// The first call to Register fails, but the second is successful.
	r.On("Register", "registration-fails").Return("", errors.New("error during registration")).Once()
	r.On("Register", "registration-fails").Return("registration-fails", nil).Once()
	r.On("ConfigureRuntimeAgent", "kubeconfig-data-registration-fails", "registration-fails").Return(nil)

	r.On("Register", "empty-kubeconfig").Return("id-empty-kubeconfig", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-data-empty-kubeconfig", "id-empty-kubeconfig").Return(nil)
}
