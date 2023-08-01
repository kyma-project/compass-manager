package director

import (
	"fmt"

	directorApperrors "github.com/kyma-incubator/compass/components/director/pkg/apperrors"
	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	"github.com/kyma-incubator/compass/components/director/pkg/graphql/graphqlizer"
	log "github.com/sirupsen/logrus"

	"github.com/kyma-project/compass-manager/internal/apperrors"
	gql "github.com/kyma-project/compass-manager/internal/graphql"
	"github.com/kyma-project/compass-manager/internal/oauth"
	gcli "github.com/kyma-project/control-plane/components/provisioner/third_party/machinebox/graphql"
)

const (
	AuthorizationHeader = "Authorization"
	TenantHeader        = "Tenant"
)

//go:generate mockery --name=DirectorClient
type DirectorClient interface {
	//CreateRuntime(config *gqlschema.RuntimeInput, tenant string) (string, apperrors.AppError)
	GetRuntime(id, tenant string) (graphql.RuntimeExt, apperrors.AppError)
	//UpdateRuntime(id string, config *graphql.RuntimeUpdateInput, tenant string) apperrors.AppError
	//DeleteRuntime(id, tenant string) apperrors.AppError
	//SetRuntimeStatusCondition(id string, statusCondition graphql.RuntimeStatusCondition, tenant string) apperrors.AppError
	//GetConnectionToken(id, tenant string) (graphql.OneTimeTokenForRuntimeExt, apperrors.AppError)
	//RuntimeExists(gardenerClusterName, tenant string) (bool, apperrors.AppError)
}

type directorClient struct {
	gqlClient     gql.Client
	queryProvider queryProvider
	graphqlizer   graphqlizer.Graphqlizer
	token         oauth.Token
	oauthClient   oauth.Client
}

func NewDirectorClient(gqlClient gql.Client, oauthClient oauth.Client) DirectorClient {
	return &directorClient{
		gqlClient:     gqlClient,
		oauthClient:   oauthClient,
		queryProvider: queryProvider{},
		graphqlizer:   graphqlizer.Graphqlizer{},
		token:         oauth.Token{},
	}
}

func (cc *directorClient) GetRuntime(id, tenant string) (graphql.RuntimeExt, apperrors.AppError) {
	log.Infof("Getting Runtime from Director service")

	runtimeQuery := cc.queryProvider.getRuntimeQuery(id)

	var response GetRuntimeResponse
	err := cc.executeDirectorGraphQLCall(runtimeQuery, tenant, &response)
	if err != nil {
		return graphql.RuntimeExt{}, err.Append("Failed to get runtime %s from Director", id)
	}
	if response.Result == nil {
		return graphql.RuntimeExt{}, apperrors.Internal("Failed to get runtime %s from Director: received nil response.", id).SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorNilResponse)
	}
	if response.Result.ID != id {
		return graphql.RuntimeExt{}, apperrors.Internal("Failed to get runtime %s from Director: received unexpected RuntimeID", id).SetComponent(apperrors.ErrCompassDirector).SetReason(apperrors.ErrDirectorRuntimeIDMismatch)
	}

	log.Infof("Successfully got Runtime %s from Director for tenant %s", id, tenant)
	return *response.Result, nil
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

func (cc *directorClient) executeDirectorGraphQLCall(directorQuery string, tenant string, response interface{}) apperrors.AppError {
	if cc.token.EmptyOrExpired() {
		log.Infof("Refreshing token to access Director Service")
		if err := cc.getToken(); err != nil {
			return err
		}
	}

	req := gcli.NewRequest(directorQuery)
	req.Header.Set(AuthorizationHeader, fmt.Sprintf("Bearer %s", cc.token.AccessToken))
	req.Header.Set(TenantHeader, tenant)

	if err := cc.gqlClient.Do(req, response); err != nil {
		if egErr, ok := err.(gcli.ExtendedError); ok {
			return mapDirectorErrorToProvisionerError(egErr).Append("Failed to execute GraphQL request to Director")
		}
		return apperrors.Internal("Failed to execute GraphQL request to Director: %v", err)
	}

	return nil
}

func mapDirectorErrorToProvisionerError(egErr gcli.ExtendedError) apperrors.AppError {
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
	case directorApperrors.NotFound, directorApperrors.NotUnique, directorApperrors.InvalidData,
		directorApperrors.InvalidOperation:
		err = apperrors.BadRequest(egErr.Error())
	case directorApperrors.TenantRequired, directorApperrors.TenantNotFound:
		err = apperrors.InvalidTenant(egErr.Error())
	default:
		err = apperrors.Internal("Did not recognize the error code from the error response. Original error: %v", egErr)
	}

	return err.SetComponent(apperrors.ErrCompassDirector).SetReason(reason)
}
