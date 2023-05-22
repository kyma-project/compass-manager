package controllers

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	operatorv1beta1 "github.com/kyma-project/compass-manager/api/v1beta1"
	"github.com/kyma-project/compass-manager/controllers/mocks"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	//+kubebuilder:scaffold:imports
)

type CompassManagerSuite struct {
	suite.Suite
	cfg          *rest.Config
	k8sClient    client.Client
	testEnv      *envtest.Environment
	mockRegister *mocks.Registrator
}

func (cm *CompassManagerSuite) SetupSuite() {
	cm.T().Logf("%s", "starting the test suite")
	cm.T().Logf("%s", "bootstrapping test environment")

	cm.testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "test")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cm.cfg, err = cm.testEnv.Start()
	cm.Require().NoError(err)
	cm.Require().NotNil(cm.cfg)

	err = operatorv1beta1.AddToScheme(scheme.Scheme)
	cm.Require().NoError(err)
	err = kyma.AddToScheme(scheme.Scheme)
	cm.Require().NoError(err)

	//+kubebuilder:scaffold:scheme

	cm.k8sClient, err = client.New(cm.cfg, client.Options{Scheme: scheme.Scheme})
	cm.Require().NoError(err)
	cm.Require().NotNil(cm.k8sClient)

	k8sManager, err := ctrl.NewManager(cm.cfg, ctrl.Options{Scheme: scheme.Scheme})
	cm.Require().NoError(err)

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	cm.mockRegister = &mocks.Registrator{}
	prepareMockFunctions(cm.mockRegister)

	compassManager := NewCompassManagerReconciler(k8sManager, log, cm.mockRegister)

	err = compassManager.SetupWithManager(k8sManager)
	cm.Require().NoError(err)

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = k8sManager.Start(ctx)
		cm.Require().NoError(err, "failed to run manager")
	}()
}

func TestCompassManagerSuite(t *testing.T) {
	suite.Run(t, new(CompassManagerSuite))
}

func (cm *CompassManagerSuite) TearDownSuite() {
	cm.T().Logf("%s", "tearing down the test environment")
	err := (func() (err error) {
		// Need to sleep if the first stop fails due to a bug:
		// https://github.com/kubernetes-sigs/controller-runtime/issues/1571
		sleepTime := 1 * time.Millisecond
		for i := 0; i < 12; i++ { // Exponentially sleep up to ~4s
			if err = cm.testEnv.Stop(); err == nil {
				return
			}
			sleepTime *= 2
			time.Sleep(sleepTime)
		}
		return
	})()
	cm.Require().NoError(err)
}

func prepareMockFunctions(r *mocks.Registrator) {
	r.On("Register", "all-good").Return("all-good", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-all-good").Return(nil)

	r.On("Register", "configure-fails").Return("configure-fails", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-configure-fails").Return(errors.New("error during configuration of Compass Runtime Agent CR"))

	r.On("Register", "registration-fails").Return("", errors.New("error during registration"))

	r.On("Register", "insignificant-field").Return("insignificant-field", nil)
	r.On("ConfigureRuntimeAgent", "kubeconfig-insignificant-field").Return(nil)
}
