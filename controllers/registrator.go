package controllers

import (
	"math/rand"
	"time"

	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	"github.com/kyma-project/compass-manager/internal/apperrors"
	"github.com/kyma-project/compass-manager/internal/director"
	"github.com/kyma-project/compass-manager/internal/util"
	"github.com/kyma-project/compass-manager/pkg/gqlschema"
	"github.com/sirupsen/logrus"
)

const nameIDLen = 4

type CompassRegistrator struct {
	Client director.Client
	Log    *logrus.Logger
}

func NewCompassRegistator(directorClient director.Client, log *logrus.Logger) *CompassRegistrator {
	return &CompassRegistrator{
		Client: directorClient,
		Log:    log,
	}
}

func (r *CompassRegistrator) RegisterInCompass(compassRuntimeLabels map[string]interface{}) (string, error) {
	var runtimeID string
	runtimeInput, err := createRuntimeInput(compassRuntimeLabels)
	if err != nil {
		return "", err
	}

	err = util.RetryOnError(5*time.Second, 3, "Error while registering runtime in Director: %s", func() (err apperrors.AppError) {
		runtimeID, err = r.Client.CreateRuntime(runtimeInput, compassRuntimeLabels["global_account_id"].(string))
		return
	})

	if err != nil {
		return "", err
	}

	return runtimeID, nil
}

func (r *CompassRegistrator) RefreshCompassToken(compassID, globalAccount string) (graphql.OneTimeTokenForRuntimeExt, error) {
	var token graphql.OneTimeTokenForRuntimeExt
	err := util.RetryOnError(5*time.Second, 3, "Error while refreshing OneTime token in Director: %s", func() (err apperrors.AppError) {
		token, err = r.Client.GetConnectionToken(compassID, globalAccount)
		return
	})

	if err != nil {
		return graphql.OneTimeTokenForRuntimeExt{}, err
	}

	return token, nil
}

func createRuntimeInput(compassRuntimeLabels map[string]interface{}) (*gqlschema.RuntimeInput, error) {
	runtimeInput := &gqlschema.RuntimeInput{}
	runtimeInput.Name = compassRuntimeLabels["gardenerClusterName"].(string) + "-" + generateRandomText(nameIDLen)

	err := runtimeInput.Labels.UnmarshalGQL(compassRuntimeLabels)
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
