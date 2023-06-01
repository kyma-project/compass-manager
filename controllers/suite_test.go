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

	operatorv1beta1 "github.com/kyma-project/compass-manager/api/v1beta1"
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
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "test")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = operatorv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
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

	err = cm.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

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
	r.On("Register", "all-good").Return("all-good", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-all-good").Return(nil)

	r.On("Register", "configure-fails").Return("configure-fails", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-configure-fails").Return(errors.New("error during configuration of Compass Runtime Agent CR"))

	r.On("Register", "registration-fails").Return("", errors.New("error during registration"))

	r.On("Register", "empty-kubeconfig").Return("empty-kubeconfig", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-empty-kubeconfig").Return(nil)

	r.On("Register", "insignificant-field").Return("insignificant-field", nil).Once()
	r.On("ConfigureRuntimeAgent", "kubeconfig-insignificant-field").Return(nil).Once()
}
