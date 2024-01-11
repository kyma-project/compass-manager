package controllers

import (
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func NewDryRunner(log *logrus.Logger) *DryRunner {
	return &DryRunner{log: log}
}

type DryRunner struct {
	log *logrus.Logger
}

func (nc DryRunner) ConfigureCompassRuntimeAgent(kubeconfig []byte, compassRuntimeID, globalAccount string) error {
	nc.log.Infof("[DRY] Configure runtime %s for GA %s", compassRuntimeID, globalAccount)
	return nil
}

func (nr DryRunner) RegisterInCompass(compassRuntimeLabels map[string]interface{}) (string, error) {
	compassID := uuid.New().String()
	nr.log.Infof("[DRY] Register runtime %s: %s", compassRuntimeLabels["global_account_id"], compassID)
	return compassID, nil
}
func (nr DryRunner) DeregisterFromCompass(compassID, globalAccount string) error {
	nr.log.Infof("[DRY] Register runtime, GA: %s Compass ID: %s", globalAccount, compassID)
	return nil
}
