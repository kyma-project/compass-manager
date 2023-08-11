package controllers

import (
	"github.com/kyma-project/compass-manager/internal/apperrors"
	"github.com/kyma-project/compass-manager/internal/director"
	"github.com/kyma-project/compass-manager/internal/util"
	"github.com/kyma-project/compass-manager/pkg/gqlschema"
	"github.com/sirupsen/logrus"
	"math/rand"
	"time"
)

const nameIDLen = 4

type CompassRegistrator struct {
	DirectorClient director.DirectorClient
	Log            *logrus.Logger
}

func NewCompassRegistator(directorClient director.DirectorClient, log *logrus.Logger) *CompassRegistrator {
	return &CompassRegistrator{
		DirectorClient: directorClient,
		Log:            log,
	}
}

func (r *CompassRegistrator) ConfigureRuntimeAgent(kubeconfig string, runtimeID string) error {
	return nil
}

func (r *CompassRegistrator) Register(kymaLabels map[string]string) (string, error) {

	var runtimeID string
	r.Log.Infof("KymaLavels: %s", kymaLabels)
	runtimeInput, err := createRuntimeInput(kymaLabels)
	if err != nil {
		return "", err
	}

	err = util.RetryOnError(5*time.Second, 3, "Error while registering runtime in Director: %s", func() (err apperrors.AppError) {
		runtimeID, err = r.DirectorClient.CreateRuntime(runtimeInput, kymaLabels[GlobalAccountIDLabel])
		return
	})

	if err != nil {
		return "", err
	}

	return runtimeID, nil
}

func createRuntimeInput(kymaLabels map[string]string) (*gqlschema.RuntimeInput, error) {

	runtimeInput := &gqlschema.RuntimeInput{}
	runtimeInput.Name = kymaLabels[ShootNameLabel] + "-" + generateRandomText(nameIDLen)

	generatedLabels := createRuntimeLabels(kymaLabels)

	err := runtimeInput.Labels.UnmarshalGQL(generatedLabels)
	if err != nil {
		return nil, err
	}

	return runtimeInput, nil
}

func generateRandomText(count int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	runes := make([]rune, count)
	for i := range runes {
		runes[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(runes)
}

func createRuntimeLabels(kymaLabels map[string]string) map[string]interface{} {

	runtimeLabels := make(map[string]interface{})

	// check if label exists?
	runtimeLabels["kyma_managed_by"] = kymaLabels[ManagedByLabel]
	runtimeLabels["director_connection_managed_by"] = "compass-manager"
	runtimeLabels["broker_instance_id"] = kymaLabels[BrokerInstanceIDLabel]
	runtimeLabels["gardenerClusterName"] = kymaLabels[ShootNameLabel]
	runtimeLabels["subaccount_id"] = kymaLabels[SubaccountIDLabel]
	runtimeLabels["global_account_id"] = kymaLabels[GlobalAccountIDLabel]
	runtimeLabels["broker_plan_id"] = kymaLabels[BrokerPlanIDLabel]
	runtimeLabels["broker_plan_name"] = kymaLabels[BrokerPlanNameLabel]

	return runtimeLabels
}
