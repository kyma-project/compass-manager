package controllers

import (
	"testing"

	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	"github.com/kyma-project/compass-manager/internal/director/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError(t *testing.T) {
	t.Run("should succeed after fetching correct Compass Token", func(t *testing.T) {
		mockDirectorClient := mocks.Client{}
		mockDirectorClient.On("GetConnectionToken", "compassID", "globalAccount").Return(graphql.OneTimeTokenForRuntimeExt{
			OneTimeTokenForRuntime: graphql.OneTimeTokenForRuntime{
				TokenWithURL: graphql.TokenWithURL{
					Token:        "dGVzdFRva2VuQmFzZWQ2NA==",
					ConnectorURL: "kyma.cloud.sap/connector/graphql",
				},
			},
		}, nil)

		configurator := NewRuntimeAgentConfigurator(&mockDirectorClient, "kyma.cloud.sap/connector/graphql", logrus.New())

		token, err := configurator.fetchCompassToken("compassID", "globalAccount")
		require.NoError(t, err)
		assert.Equal(t, "kyma.cloud.sap/connector/graphql", token.ConnectorURL)
		assert.Equal(t, "dGVzdFRva2VuQmFzZWQ2NA==", token.Token)
	})

	t.Run("should return error after fetching invalid Connector URL", func(t *testing.T) {
		mockDirectorClient := mocks.Client{}
		mockDirectorClient.On("GetConnectionToken", "compassID", "globalAccount").Return(graphql.OneTimeTokenForRuntimeExt{
			OneTimeTokenForRuntime: graphql.OneTimeTokenForRuntime{
				TokenWithURL: graphql.TokenWithURL{
					Token:        "dGVzdFRva2VuQmFzZWQ2NA==",
					ConnectorURL: "invalid.domain/connector/graphql",
				},
			},
		}, nil)

		configurator := NewRuntimeAgentConfigurator(&mockDirectorClient, "kyma.cloud.sap/connector/graphql", logrus.New())

		token, err := configurator.fetchCompassToken("compassID", "globalAccount")
		require.Error(t, err)
		require.ErrorContains(t, err, "Connector URL does not match the expected pattern")
		assert.Equal(t, token, graphql.OneTimeTokenForRuntimeExt{})
	})
	t.Run("should return error after error during decoding of Runtime Token", func(t *testing.T) {
		mockDirectorClient := mocks.Client{}
		mockDirectorClient.On("GetConnectionToken", "compassID", "globalAccount").Return(graphql.OneTimeTokenForRuntimeExt{
			OneTimeTokenForRuntime: graphql.OneTimeTokenForRuntime{
				TokenWithURL: graphql.TokenWithURL{
					Token:        "not-base64-encoded",
					ConnectorURL: "kyma.cloud.sap/connector/graphql",
				},
			},
		}, nil)

		configurator := NewRuntimeAgentConfigurator(&mockDirectorClient, "kyma.cloud.sap/connector/graphql", logrus.New())

		token, err := configurator.fetchCompassToken("compassID", "globalAccount")
		require.Error(t, err)
		require.ErrorContains(t, err, "OneTimeToken cannot be decoded")
		assert.Equal(t, token, graphql.OneTimeTokenForRuntimeExt{})
	})
	t.Run("should return error when Runtime Token is too long", func(t *testing.T) {
		mockDirectorClient := mocks.Client{}
		mockDirectorClient.On("GetConnectionToken", "compassID", "globalAccount").Return(graphql.OneTimeTokenForRuntimeExt{
			OneTimeTokenForRuntime: graphql.OneTimeTokenForRuntime{
				TokenWithURL: graphql.TokenWithURL{
					Token:        "bm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVkbm90LWJhc2U2NC1lbmNvZGVk",
					ConnectorURL: "kyma.cloud.sap/connector/graphql",
				},
			},
		}, nil)

		configurator := NewRuntimeAgentConfigurator(&mockDirectorClient, "kyma.cloud.sap/connector/graphql", logrus.New())

		token, err := configurator.fetchCompassToken("compassID", "globalAccount")
		require.Error(t, err)
		require.ErrorContains(t, err, "OneTimeToken is too long")
		assert.Equal(t, token, graphql.OneTimeTokenForRuntimeExt{})
	})
}
