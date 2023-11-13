package controllers

import (
	"context"
	"time"

	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	"github.com/kyma-project/compass-manager/internal/apperrors"
	"github.com/kyma-project/compass-manager/internal/director"
	"github.com/kyma-project/compass-manager/internal/util"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	AgentConfigurationSecretName   = "compass-agent-configuration"
	runtimeAgentComponentNameSpace = "kyma-system"
)

type RuntimeAgentConfigurator struct {
	Client director.Client
	Log    *logrus.Logger
}

func NewRuntimeAgentConfigurator(directorClient director.Client, log *logrus.Logger) *RuntimeAgentConfigurator {
	return &RuntimeAgentConfigurator{
		Client: directorClient,
		Log:    log,
	}
}

func (r *RuntimeAgentConfigurator) ConfigureCompassRuntimeAgent(kubeconfig string, compassRuntimeID, globalAccount string) error {
	kubeClient, err := r.prepareKubeClient(kubeconfig)
	if err != nil {
		return err
	}

	token, err := r.fetchCompassToken(compassRuntimeID, globalAccount)
	if err != nil {
		return err
	}

	err = r.upsertCompassRuntimeAgentSecret(kubeClient, token, compassRuntimeID, globalAccount)
	if err != nil {
		return err
	}
	return nil
}

func (r *RuntimeAgentConfigurator) upsertCompassRuntimeAgentSecret(kubeClient kubernetes.Interface, token graphql.OneTimeTokenForRuntimeExt, runtimeID, globalAccount string) error {
	configurationData := map[string]string{
		"CONNECTOR_URL": token.ConnectorURL,
		"RUNTIME_ID":    runtimeID,
		"TENANT":        globalAccount,
		"TOKEN":         token.Token,
	}

	secret := &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      AgentConfigurationSecretName,
			Namespace: runtimeAgentComponentNameSpace,
		},
		StringData: configurationData,
	}

	secretInterface := kubeClient.CoreV1().Secrets(runtimeAgentComponentNameSpace)

	_, err := secretInterface.Get(context.TODO(), AgentConfigurationSecretName, meta.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			_, err = secretInterface.Create(context.TODO(), secret, meta.CreateOptions{})
			return err
		}
	}
	_, err = secretInterface.Update(context.TODO(), secret, meta.UpdateOptions{})
	return err
}

func (r *RuntimeAgentConfigurator) prepareKubeClient(kubeconfig string) (kubernetes.Interface, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func (r *RuntimeAgentConfigurator) fetchCompassToken(compassID, globalAccount string) (graphql.OneTimeTokenForRuntimeExt, error) {
	var token graphql.OneTimeTokenForRuntimeExt
	err := util.RetryOnError(retryTime*time.Second, attempts, "Error while refreshing OneTime token in Director: %s", func() (err apperrors.AppError) {
		token, err = r.Client.GetConnectionToken(compassID, globalAccount)
		return
	})

	if err != nil {
		return graphql.OneTimeTokenForRuntimeExt{}, err
	}

	return token, nil
}
