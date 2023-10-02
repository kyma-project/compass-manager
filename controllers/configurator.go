package controllers

import "github.com/sirupsen/logrus"

type RuntimeAgentConfigurator struct {
	Log *logrus.Logger
}

func NewRuntimeAgentConfigurator(log *logrus.Logger) *RuntimeAgentConfigurator {
	return &RuntimeAgentConfigurator{Log: log}
}

func (r *RuntimeAgentConfigurator) ConfigureCompassRuntimeAgent(kubeconfig string, runtimeID string) error { //nolint:revive
	return nil
}

func (r *RuntimeAgentConfigurator) UpdateCompassRuntimeAgent(kubeconfig string) error { //nolint:revive
	return nil
}
