package controllers

import (
	"github.com/kyma-project/compass-manager/internal/director"
	"github.com/sirupsen/logrus"
)

type CompassRegistrator struct {
	Client director.DirectorClient
	Log    *logrus.Logger
}

func (r *CompassRegistrator) ConfigureRuntimeAgent(kubeconfig string, runtimeID string) error {
	return nil
}

func (r *CompassRegistrator) Register(name, globalAccount string) (string, error) {
	runs, err := r.Client.GetRuntime(name, globalAccount)
	if err != nil {
		return "", err
	}
	return runs.ID, nil
}
