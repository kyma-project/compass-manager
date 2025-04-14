package director

import (
	"errors"
	"fmt"
	"testing"
	"time"

	directorApperrors "github.com/kyma-incubator/compass/components/director/pkg/apperrors"
	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	"github.com/kyma-project/compass-manager/internal/apperrors"
	gql "github.com/kyma-project/compass-manager/internal/graphql"
	"github.com/kyma-project/compass-manager/internal/oauth"
	oauthmocks "github.com/kyma-project/compass-manager/internal/oauth/mocks"
	"github.com/kyma-project/compass-manager/internal/util"
	"github.com/kyma-project/compass-manager/pkg/gqlschema"
	gcli "github.com/kyma-project/compass-manager/third_party/machinebox/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	compassTestingID   = "4366e452-2ffb-435d-abbd-81cf5d3965c9"
	compassTestingName = "Runtime Test name"
	validTokenValue    = "12345"
	globalAccountValue = "3e64ebae-38b5-46a0-b1ed-9ccee153a0ae"
	oneTimeToken       = "54321"
	connectorURL       = "https://kyma.cx/connector/graphql"

	expectedRegisterRuntimeQuery = `mutation {
	result: registerRuntime(in: {
		name: "Runtime Test name",
		description: "runtime description",
	}) { id } }`

	expectedOneTimeTokenQuery = `mutation {
	result: requestOneTimeTokenForRuntime(id: "4366e452-2ffb-435d-abbd-81cf5d3965c9") {
		token connectorURL
}}`

	expectedGetRuntimeQuery = `query {
    result: runtime(id: "4366e452-2ffb-435d-abbd-81cf5d3965c9") {
         id name description labels
}}`

	expectedDeleteRuntimeQuery = `mutation {
	result: unregisterRuntime(id: "4366e452-2ffb-435d-abbd-81cf5d3965c9") {
		id
}}`
)

var (
	futureExpirationTime = time.Now().Add(time.Duration(60) * time.Minute).Unix()      //nolint:gochecknoglobals
	passedExpirationTime = time.Now().Add(time.Duration(60) * time.Minute * -1).Unix() //nolint:gochecknoglobals
)

func TestDirectorClient_RuntimeRegistering(t *testing.T) {
	expectedRequest := gcli.NewRequest(expectedRegisterRuntimeQuery)
	expectedRequest.Header.Set(AuthorizationHeader, fmt.Sprintf("Bearer %s", validTokenValue))
	expectedRequest.Header.Set(TenantHeader, globalAccountValue)

	inputDescription := "runtime description"

	runtimeInput := &gqlschema.RuntimeInput{
		Name:        compassTestingName,
		Description: &inputDescription,
	}

	t.Run("Should register runtime and return new runtime ID when the Director access token is valid", func(t *testing.T) {
		// given
		responseDescription := "runtime description"
		expectedResponse := &graphql.Runtime{
			ID:          compassTestingID,
			Name:        compassTestingName,
			Description: &responseDescription,
		}

		expectedID := compassTestingID

		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*CreateRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = expectedResponse
		})

		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		receivedRuntimeID, err := configClient.CreateRuntime(runtimeInput, globalAccountValue)

		// then
		assert.NoError(t, err)
		assert.Equal(t, expectedID, receivedRuntimeID)
	})

	t.Run("Should not register runtime and return an error when the Director access token is empty", func(t *testing.T) {
		// given
		token := oauth.Token{
			AccessToken: "",
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		receivedRuntimeID, err := configClient.CreateRuntime(runtimeInput, globalAccountValue)

		// then
		assert.Error(t, err)
		assert.Empty(t, receivedRuntimeID)
	})

	t.Run("Should not register runtime and return an error when the Director access token is expired", func(t *testing.T) {
		// given
		expiredToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  passedExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(expiredToken, nil)

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		receivedRuntimeID, err := configClient.CreateRuntime(runtimeInput, globalAccountValue)

		// then
		assert.Error(t, err)
		assert.Empty(t, receivedRuntimeID)
	})

	t.Run("Should not register Runtime and return error when the client fails to get an access token for Director", func(t *testing.T) {
		// given
		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(oauth.Token{}, apperrors.Internalf("Failed token error"))

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		receivedRuntimeID, err := configClient.CreateRuntime(runtimeInput, globalAccountValue)

		// then
		assert.Error(t, err)
		util.CheckErrorType(t, err, apperrors.CodeInternal)
		assert.Empty(t, receivedRuntimeID)
	})

	t.Run("Should not register Runtime and return error when the Runtime ID from Director is not in UUID format", func(t *testing.T) {
		// given
		responseDescription := "runtime description"
		expectedResponse := &graphql.Runtime{
			ID:          "non-uuid-format",
			Name:        compassTestingName,
			Description: &responseDescription,
		}

		expectedID := ""

		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*CreateRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = expectedResponse
		})

		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		receivedRuntimeID, err := configClient.CreateRuntime(runtimeInput, globalAccountValue)

		// then
		assert.Error(t, err)
		assert.Equal(t, expectedID, receivedRuntimeID)
	})

	t.Run("Should return error when the result of the call to Director service is nil", func(t *testing.T) {
		// given
		validToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(validToken, nil)

		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*CreateRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = nil
		})

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		receivedRuntimeID, err := configClient.CreateRuntime(runtimeInput, globalAccountValue)

		// then
		assert.Error(t, err)
		assert.Empty(t, receivedRuntimeID)
	})

	t.Run("Should return error when Director fails to register Runtime ", func(t *testing.T) {
		// given
		validToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(validToken, nil)

		gqlClient := gql.NewQueryAssertClient(t, errors.New("error"), []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*CreateRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = nil
		})

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		receivedRuntimeID, err := configClient.CreateRuntime(runtimeInput, globalAccountValue)

		// then
		assert.Error(t, err)
		assert.Empty(t, receivedRuntimeID)
	})
}

func TestDirectorClient_RuntimeUnregistering(t *testing.T) {
	expectedRequest := gcli.NewRequest(expectedDeleteRuntimeQuery)
	expectedRequest.Header.Set(AuthorizationHeader, fmt.Sprintf("Bearer %s", validTokenValue))
	expectedRequest.Header.Set(TenantHeader, globalAccountValue)

	t.Run("Should unregister runtime of given ID and return no error when the Director access token is valid", func(t *testing.T) {
		// given
		responseDescription := "runtime description"
		expectedResponse := &graphql.Runtime{
			ID:          compassTestingID,
			Name:        compassTestingName,
			Description: &responseDescription,
		}

		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*DeleteRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = expectedResponse
		})

		validToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(validToken, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		err := configClient.DeleteRuntime(compassTestingID, globalAccountValue)

		// then
		assert.NoError(t, err)
	})

	t.Run("Should not unregister runtime and return an error when the Director access token is empty", func(t *testing.T) {
		// given
		emptyToken := oauth.Token{
			AccessToken: "",
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(emptyToken, nil)

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		err := configClient.DeleteRuntime(compassTestingID, globalAccountValue)

		// then
		assert.Error(t, err)
	})

	t.Run("Should not unregister register runtime and return an error when the Director access token is expired", func(t *testing.T) {
		// given
		expiredToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  passedExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(expiredToken, nil)

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		err := configClient.DeleteRuntime(compassTestingID, globalAccountValue)

		// then
		assert.Error(t, err)
	})

	t.Run("Should not unregister Runtime and return error when the client fails to get an access token for Director", func(t *testing.T) {
		// given
		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(oauth.Token{}, apperrors.Internalf("Failed token error"))

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		err := configClient.DeleteRuntime(compassTestingID, globalAccountValue)

		// then
		assert.Error(t, err)
		util.CheckErrorType(t, err, apperrors.CodeInternal)
	})

	t.Run("Should return error when the result of the call to Director service is nil", func(t *testing.T) {
		// given
		validToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(validToken, nil)

		// given
		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*DeleteRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = nil
		})

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		err := configClient.DeleteRuntime(compassTestingID, globalAccountValue)

		// then
		assert.Error(t, err)
	})

	t.Run("Should return error when Director fails to delete Runtime", func(t *testing.T) {
		// given
		gqlClient := gql.NewQueryAssertClient(t, errors.New("error"), []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*DeleteRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = nil
		})

		validToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(validToken, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		err := configClient.DeleteRuntime(compassTestingID, globalAccountValue)

		// then
		assert.Error(t, err)
	})

	// unusual and strange case
	t.Run("Should return error when Director returns bad ID after Deleting", func(t *testing.T) {
		// given
		responseDescription := "runtime description"
		expectedResponse := &graphql.Runtime{
			ID:          "BadId",
			Name:        compassTestingName,
			Description: &responseDescription,
		}

		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*DeleteRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = expectedResponse
		})

		validToken := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(validToken, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		err := configClient.DeleteRuntime(compassTestingID, globalAccountValue)

		// then
		assert.Error(t, err)
	})
}

func TestDirectorClient_GetConnectionToken(t *testing.T) {
	expectedRequest := gcli.NewRequest(expectedOneTimeTokenQuery)
	expectedRequest.Header.Set(AuthorizationHeader, fmt.Sprintf("Bearer %s", validTokenValue))
	expectedRequest.Header.Set(TenantHeader, globalAccountValue)

	t.Run("Should return OneTimeToken when Oauth Token is valid", func(t *testing.T) {
		// given
		expectedResponse := &graphql.OneTimeTokenForRuntimeExt{
			OneTimeTokenForRuntime: graphql.OneTimeTokenForRuntime{
				TokenWithURL: graphql.TokenWithURL{
					Token:        oneTimeToken,
					ConnectorURL: connectorURL,
				},
			},
		}

		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*OneTimeTokenResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = expectedResponse
		})

		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		receivedOneTimeToken, err := configClient.GetConnectionToken(compassTestingID, globalAccountValue)

		// then
		require.NoError(t, err)
		require.NotEmpty(t, receivedOneTimeToken)
		assert.Equal(t, oneTimeToken, receivedOneTimeToken.Token)
		assert.Equal(t, connectorURL, receivedOneTimeToken.ConnectorURL)
	})

	t.Run("Should return error when Oauth Token is empty", func(t *testing.T) {
		// given
		token := oauth.Token{
			AccessToken: "",
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		receivedOneTimeToken, err := configClient.GetConnectionToken(compassTestingID, globalAccountValue)

		// then
		require.Error(t, err)
		require.Empty(t, receivedOneTimeToken)
	})

	t.Run("Should return error when Oauth Token is expired", func(t *testing.T) {
		// given
		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  passedExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		receivedOneTimeToken, err := configClient.GetConnectionToken(compassTestingID, globalAccountValue)

		// then
		require.Error(t, err)
		require.Empty(t, receivedOneTimeToken)
	})

	t.Run("Should return error when Director call returns nil reponse", func(t *testing.T) {
		// given
		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*OneTimeTokenResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
		})

		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		receivedOneTimeToken, err := configClient.GetConnectionToken(compassTestingID, globalAccountValue)

		// then
		require.Error(t, err)
		require.Empty(t, receivedOneTimeToken)
	})
}

func TestDirectorClient_GetRuntime(t *testing.T) {
	expectedRequest := gcli.NewRequest(expectedGetRuntimeQuery)
	expectedRequest.Header.Set(AuthorizationHeader, fmt.Sprintf("Bearer %s", validTokenValue))
	expectedRequest.Header.Set(TenantHeader, globalAccountValue)

	t.Run("should return Runtime", func(t *testing.T) {
		// given
		expectedResponse := &graphql.RuntimeExt{
			Runtime: graphql.Runtime{
				ID:   compassTestingID,
				Name: compassTestingName,
			},
		}

		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*GetRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = expectedResponse
		})

		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		runtime, err := configClient.GetRuntime(compassTestingID, globalAccountValue)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedResponse.Name, runtime.Name)
		assert.Equal(t, expectedResponse.ID, runtime.ID)
	})

	t.Run("should return error when access token is empty", func(t *testing.T) {
		// given
		emptyToken := oauth.Token{
			AccessToken: "",
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(emptyToken, nil)

		configClient := NewDirectorClient(nil, mockedOAuthClient)

		// when
		runtime, err := configClient.GetRuntime(compassTestingID, globalAccountValue)

		// then
		assert.Error(t, err)
		assert.Empty(t, runtime)
	})

	t.Run("should return error when Director returns nil response", func(t *testing.T) {
		// given
		gqlClient := gql.NewQueryAssertClient(t, nil, []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*GetRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = nil
		})

		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		runtime, err := configClient.GetRuntime(compassTestingID, globalAccountValue)

		// then
		require.Error(t, err)
		assert.Empty(t, runtime)
	})

	t.Run("should return error when Director fails to get Runtime", func(t *testing.T) {
		// given
		gqlClient := gql.NewQueryAssertClient(t, errors.New("error"), []*gcli.Request{expectedRequest}, func(t *testing.T, r interface{}) {
			cfg, ok := r.(*GetRuntimeResponse)
			require.True(t, ok)
			assert.Empty(t, cfg.Result)
			cfg.Result = nil
		})

		token := oauth.Token{
			AccessToken: validTokenValue,
			Expiration:  futureExpirationTime,
		}

		mockedOAuthClient := &oauthmocks.Client{}
		mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

		configClient := NewDirectorClient(gqlClient, mockedOAuthClient)

		// when
		runtime, err := configClient.GetRuntime(compassTestingID, globalAccountValue)

		// then
		require.Error(t, err)
		assert.Empty(t, runtime)
	})
}

type testGraphQLError struct {
	Message         string
	ErrorExtensions map[string]interface{}
}

func (e testGraphQLError) Error() string {
	return "graphql: " + e.Message
}

func (e testGraphQLError) Extensions() map[string]interface{} {
	return e.ErrorExtensions
}

func TestDirectorClient_MapDirectorErrors(t *testing.T) {
	// given
	expectedRequest := gcli.NewRequest(expectedRegisterRuntimeQuery)
	expectedRequest.Header.Set(AuthorizationHeader, fmt.Sprintf("Bearer %s", validTokenValue))
	expectedRequest.Header.Set(TenantHeader, globalAccountValue)

	inputDescription := "runtime description"
	runtimeInput := &gqlschema.RuntimeInput{
		Name:        compassTestingName,
		Description: &inputDescription,
	}

	token := oauth.Token{
		AccessToken: validTokenValue,
		Expiration:  futureExpirationTime,
	}

	testcases := []struct {
		description             string
		directorErrorExtensions map[string]interface{}
		provisionerErrorCode    apperrors.ErrCode
		internalErrorCode       apperrors.CauseCode
		provisionerErrorMessage string
	}{
		{
			"Should map Director Internalf Error to Provisioner Internalf Error",
			map[string]interface{}{"error_code": float64(directorApperrors.InternalError)},
			apperrors.CodeInternal,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Unknown Error to Provisioner Internalf Error",
			map[string]interface{}{"error_code": float64(directorApperrors.UnknownError)},
			apperrors.CodeInternal,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Insufficient Scopes Error to Provisioner Bad Gateway Error",
			map[string]interface{}{"error_code": float64(directorApperrors.InsufficientScopes)},
			apperrors.CodeBadGateway,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Unauthorized Error to Provisioner Bad Gateway Error",
			map[string]interface{}{"error_code": float64(directorApperrors.Unauthorized)},
			apperrors.CodeBadGateway,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Not Found Error to Provisioner Bad Request Error",
			map[string]interface{}{"error_code": float64(directorApperrors.NotFound)},
			apperrors.CodeBadRequest,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Not Unique Error to Provisioner Bad Request Error",
			map[string]interface{}{"error_code": float64(directorApperrors.NotUnique)},
			apperrors.CodeBadRequest,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Invalid Data Error to Provisioner Bad Request Error",
			map[string]interface{}{"error_code": float64(directorApperrors.InvalidData)},
			apperrors.CodeBadRequest,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Tenant Required Error to Provisioner Bad Request Error",
			map[string]interface{}{"error_code": float64(directorApperrors.TenantRequired)},
			apperrors.CodeBadRequest,
			apperrors.GlobalAccountNotFound,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Tenant Not Found Error to Provisioner Bad Request Error",
			map[string]interface{}{"error_code": float64(directorApperrors.TenantNotFound)},
			apperrors.CodeBadRequest,
			apperrors.GlobalAccountNotFound,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should map Director Invalid Operation Error to Provisioner Bad Request Error",
			map[string]interface{}{"error_code": float64(directorApperrors.InvalidOperation)},
			apperrors.CodeBadRequest,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, graphql: some error",
		},
		{
			"Should return Internalf Error if failed to find error code in the Director Error",
			map[string]interface{}{"something_else": float64(directorApperrors.InvalidOperation)},
			apperrors.CodeInternal,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, Failed to read the error code from the error response. Original error: graphql: some error",
		},
		{
			"Should return Internalf Error if failed to cast error code from the Director Error",
			map[string]interface{}{"error_code": "not a float64"},
			apperrors.CodeInternal,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, Failed to cast the error code from the error response. Original error: graphql: some error",
		},
		{
			"Should return Internalf Error if failed to recognize the Director Error code",
			map[string]interface{}{"error_code": float64(123)},
			apperrors.CodeInternal,
			apperrors.Unknown,
			"Failed to register runtime in Director. Request failed, Failed to execute GraphQL request to Director, Did not recognize the error code from the error response. Original error: graphql: some error",
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.description, func(t *testing.T) {
			// given
			directorError := &testGraphQLError{
				Message:         "some error",
				ErrorExtensions: testcase.directorErrorExtensions,
			}

			gqlClient := gql.NewQueryAssertClient(t, directorError, []*gcli.Request{expectedRequest})

			mockedOAuthClient := &oauthmocks.Client{}
			mockedOAuthClient.On("GetAuthorizationToken").Return(token, nil)

			directorClient := NewDirectorClient(gqlClient, mockedOAuthClient)

			// when
			_, err := directorClient.CreateRuntime(runtimeInput, globalAccountValue)

			// then
			require.Error(t, err)
			assert.Equal(t, testcase.provisionerErrorCode, err.Code())
			assert.Equal(t, testcase.internalErrorCode, err.Cause())
			assert.Equal(t, testcase.provisionerErrorMessage, err.Error())
		})
	}
}
