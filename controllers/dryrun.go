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

func (dr DryRunner) ConfigureCompassRuntimeAgent(_ []byte, compassRuntimeID, globalAccount string) error {
	dr.log.Infof("[DRY] Configure runtime %s for GA %s", compassRuntimeID, globalAccount)
	return nil
}

func (dr DryRunner) RegisterInCompass(compassRuntimeLabels map[string]interface{}) (string, error) {
	compassID := uuid.New().String()
	dr.log.Infof("[DRY] Register runtime %s: %s", compassRuntimeLabels["global_account_id"], compassID)
	return compassID, nil
}
func (dr DryRunner) DeregisterFromCompass(compassID, globalAccount string) error {
	dr.log.Infof("[DRY] Register runtime, GA: %s Compass ID: %s", globalAccount, compassID)
	return nil
}
