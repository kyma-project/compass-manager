package director

import (
	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
)

type CreateRuntimeResponse struct {
	Result *graphql.Runtime `json:"result"`
}

type GetRuntimeResponse struct {
	Result *graphql.RuntimeExt `json:"result"`
}

type DeleteRuntimeResponse struct {
	Result *graphql.Runtime `json:"result"`
}

type OneTimeTokenResponse struct {
	Result *graphql.OneTimeTokenForRuntimeExt `json:"result"`
}
