package director

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	directorApperrors "github.com/kyma-incubator/compass/components/director/pkg/apperrors"
	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	"github.com/kyma-incubator/compass/components/director/pkg/graphql/graphqlizer"
	"github.com/kyma-project/compass-manager/internal/apperrors"
	gql "github.com/kyma-project/compass-manager/internal/graphql"
	"github.com/kyma-project/compass-manager/internal/oauth"
	"github.com/kyma-project/compass-manager/pkg/gqlschema"
	gcli "github.com/kyma-project/compass-manager/third_party/machinebox/graphql"
	log "github.com/sirupsen/logrus"
)

const (
	AuthorizationHeader = "Authorization"
	TenantHeader        = "Tenant"
)

//go:generate mockery --name=Client
type Client interface {
	CreateRuntime(config *gqlschema.RuntimeInput, globalAccount string) (string, apperrors.AppError)
	GetRuntime(compassID, globalAccount string) (graphql.RuntimeExt, apperrors.AppError)
	GetConnectionToken(compassID, globalAccount string) (graphql.OneTimeTokenForRuntimeExt, apperrors.AppError)
	DeleteRuntime(compassID, globalAccount string) apperrors.AppError
}

type directorClient struct {
	gqlClient     gql.Client
	queryProvider queryProvider
	graphqlizer   graphqlizer.Graphqlizer
	token         oauth.Token
	oauthClient   oauth.Client
}

func NewDirectorClient(gqlClient gql.Client, oauthClient oauth.Client) Client {
	return &directorClient{
		gqlClient:     gqlClient,
		oauthClient:   oauthClient,
		queryProvider: queryProvider{},
		graphqlizer:   graphqlizer.Graphqlizer{},
		token:         oauth.Token{},
	}
}

func (cc *directorClient) CreateRuntime(config *gqlschema.RuntimeInput, globalAccount string) (string, apperrors.AppError) {
	log.Infof("Registering Runtime on Director service")

	if config == nil {
		return "", apperrors.BadRequest("Cannot register runtime in Director: missing Runtime config")
	}

	var labels graphql.Labels
	if config.Labels != nil {
		l := graphql.Labels(config.Labels)
		labels = l
	}

	directorInput := graphql.RuntimeRegisterInput{
		Name:        config.Name,
		Description: config.Description,
		Labels:      labels,
	}

	runtimeInput, err := cc.graphqlizer.RuntimeRegisterInputToGQL(directorInput)
	if err != nil {
		log.Infof("Failed to create graphQLized Runtime input")
		return "", apperrors.Internal("Failed to create graphQLized Runtime input: %s", err.Error()).SetComponent(apperrors.ErrCompassDirectorClient).SetReason(apperrors.ErrDirectorClientGraphqlizer)
	}

	runtimeQuery := cc.queryProvider.createRuntimeMutation(runtimeInput)

	var response CreateRuntimeResponse
	appErr := cc.executeDirectorGraphQLCall(runtimeQuery, globalAccount, &response, false)
	if appErr != nil {
		return "", appErr.Append("Failed to register runtime in Director. Request failed")
	}

	// Nil check is necessary due to GraphQL client not checking response code
	if response.Result == nil {
		return "", apperrors.Internal("Failed to register runtime in Director: Received nil response.").SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorNilResponse)
	}

	_, err = uuid.Parse(response.Result.ID)
	if err != nil {
		return "", apperrors.Internal("Failed to register runtime in Director: Received ID is not in UUID format").SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorRuntimeIDInvalidFormat)
	}

	log.Infof("Successfully registered Runtime %s in Director for Global Account %s", config.Name, globalAccount)

	return response.Result.ID, nil
}

func (cc *directorClient) GetRuntime(compassID, globalAccount string) (graphql.RuntimeExt, apperrors.AppError) {
	log.Infof("Getting Runtime from Director service")

	runtimeQuery := cc.queryProvider.getRuntimeQuery(compassID)

	var response GetRuntimeResponse
	err := cc.executeDirectorGraphQLCall(runtimeQuery, globalAccount, &response, false)
	if err != nil {
		return graphql.RuntimeExt{}, err.Append("Failed to get runtime %s from Director", compassID)
	}
	if response.Result == nil {
		return graphql.RuntimeExt{}, apperrors.Internal("Failed to get runtime %s from Director: received nil response.", compassID).SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorNilResponse)
	}
	if response.Result.ID != compassID {
		return graphql.RuntimeExt{}, apperrors.Internal("Failed to get runtime %s from Director: received unexpected RuntimeID", compassID).SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorRuntimeIDMismatch)
	}

	log.Infof("Successfully got Runtime %s from Director for Global Account %s", compassID, globalAccount)
	return *response.Result, nil
}

func (cc *directorClient) GetConnectionToken(compassID, globalAccount string) (graphql.OneTimeTokenForRuntimeExt, apperrors.AppError) {
	runtimeQuery := cc.queryProvider.requestOneTimeTokenMutation(compassID)

	var response OneTimeTokenResponse
	err := cc.executeDirectorGraphQLCall(runtimeQuery, globalAccount, &response, false)
	if err != nil {
		return graphql.OneTimeTokenForRuntimeExt{}, err.Append("Failed to get OneTimeToken for Runtime %s in Director", compassID)
	}

	if response.Result == nil {
		return graphql.OneTimeTokenForRuntimeExt{}, apperrors.Internal("Failed to get OneTimeToken for Runtime %s in Director: received nil response.", compassID).SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorNilResponse)
	}

	log.Infof("Received OneTimeToken for Runtime %s in Director for Global Account %s", compassID, globalAccount)

	return *response.Result, nil
}

func (cc *directorClient) DeleteRuntime(compassID, globalAccount string) apperrors.AppError {
	runtimeQuery := cc.queryProvider.deleteRuntimeMutation(compassID)

	var response DeleteRuntimeResponse
	err := cc.executeDirectorGraphQLCall(runtimeQuery, globalAccount, &response, true)
	if err != nil {
		if err.Cause() == apperrors.RuntimeNotFound {
			log.Infof("Runtime %s in Director for tenant %s was previously deleted", compassID, globalAccount)
			return nil
		}
		return err.Append("Failed to unregister runtime %s in Director", compassID)
	}
	// Nil check is necessary due to GraphQL client not checking response code
	if response.Result == nil {
		return apperrors.Internal("Failed to unregister runtime %s in Director: received nil response.", compassID).SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorNilResponse)
	}

	if response.Result.ID != compassID {
		return apperrors.Internal("Failed to unregister runtime %s in Director: received unexpected RuntimeID.", compassID).SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorRuntimeIDMismatch)
	}

	log.Infof("Successfully unregistered Runtime %s in Director for tenant %s", compassID, globalAccount)

	return nil
}

func (cc *directorClient) getToken() apperrors.AppError {
	token, err := cc.oauthClient.GetAuthorizationToken()
	if err != nil {
		return err.Append("Error while obtaining token")
	}

	if token.EmptyOrExpired() {
		return apperrors.Internal("Obtained empty or expired token")
	}

	cc.token = token
	return nil
}

func (cc *directorClient) executeDirectorGraphQLCall(directorQuery string, globalAccount string, response interface{}, gracefulUnregistration bool) apperrors.AppError {
	if cc.token.EmptyOrExpired() {
		log.Infof("Refreshing token to access Director Service")
		if err := cc.getToken(); err != nil {
			return err
		}
	}

	req := gcli.NewRequest(directorQuery)
	req.Header.Set(AuthorizationHeader, fmt.Sprintf("Bearer %s", cc.token.AccessToken))
	req.Header.Set(TenantHeader, globalAccount)

	if err := cc.gqlClient.Do(req, response, gracefulUnregistration); err != nil {
		var egErr gcli.ExtendedError
		if errors.As(err, &egErr) {
			return mapDirectorErrorToProvisionerError(egErr, gracefulUnregistration).Append("Failed to execute GraphQL request to Director")
		}
		return apperrors.Internal("Failed to execute GraphQL request to Director: %v", err)
	}

	return nil
}

func mapDirectorErrorToProvisionerError(egErr gcli.ExtendedError, gracefulUnregistration bool) apperrors.AppError {
	errorCodeValue, present := egErr.Extensions()["error_code"]
	if !present {
		return apperrors.Internal("Failed to read the error code from the error response. Original error: %v", egErr)
	}
	errorCode, ok := errorCodeValue.(float64)
	if !ok {
		return apperrors.Internal("Failed to cast the error code from the error response. Original error: %v", egErr)
	}

	var err apperrors.AppError
	reason := apperrors.ErrReason(directorApperrors.ErrorType(errorCode).String())

	switch directorApperrors.ErrorType(errorCode) {
	case directorApperrors.InternalError, directorApperrors.UnknownError:
		err = apperrors.Internal(egErr.Error())
	case directorApperrors.InsufficientScopes, directorApperrors.Unauthorized:
		err = apperrors.BadGateway(egErr.Error())
	case directorApperrors.NotFound:
		if gracefulUnregistration {
			err = apperrors.NotFound(egErr.Error())
			return err
		}
		err = apperrors.BadRequest(egErr.Error())
	case directorApperrors.NotUnique, directorApperrors.InvalidData,
		directorApperrors.InvalidOperation:
		err = apperrors.BadRequest(egErr.Error())
	case directorApperrors.TenantRequired, directorApperrors.TenantNotFound:
		err = apperrors.InvalidGlobalAccount(egErr.Error())
	default:
		err = apperrors.Internal("Did not recognize the error code from the error response. Original error: %v", egErr)
	}

	return err.SetComponent(apperrors.ErrCompassDirector).SetReason(reason)
}
