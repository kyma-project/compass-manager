package graphql

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	directorApperrors "github.com/kyma-incubator/compass/components/director/pkg/apperrors"
	"github.com/kyma-project/compass-manager/third_party/machinebox/graphql"
	"github.com/sirupsen/logrus"
)

const (
	timeout = 30 * time.Second
)

type ClientConstructor func(certificate *tls.Certificate, graphqlEndpoint string, enableLogging bool, insecureConfigFetch bool) (Client, error)

//go:generate mockery --name=Client
type Client interface {
	Do(req *graphql.Request, res interface{}, gracefulUnregistration bool) error
}

type client struct {
	gqlClient *graphql.Client
	logs      []string
	logging   bool
}

func NewGraphQLClient(graphqlEndpoint string, enableLogging bool, insecureSkipVerify bool) Client {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		},
	}

	gqlClient := graphql.NewClient(graphqlEndpoint, graphql.WithHTTPClient(httpClient))

	client := &client{
		gqlClient: gqlClient,
		logging:   enableLogging,
		logs:      []string{},
	}

	client.gqlClient.Log = client.addLog

	return client
}

func (c *client) Do(req *graphql.Request, res interface{}, gracefulUnregistration bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c.clearLogs()
	err := c.gqlClient.Run(ctx, req, res)
	if err != nil { //nolint:nestif
		var egErr graphql.ExtendedError
		if errors.As(err, &egErr) {
			switch gracefulUnregistration {
			case true:
				errorCodeValue, present := egErr.Extensions()["error_code"]
				if !present {
					errs := errors.New("failed to read the error code from the error response. Original error: ")
					return errors.Join(errs, egErr)
				}
				errorCode, ok := errorCodeValue.(float64)
				if !ok {
					errs := errors.New("failed to cast the error code from the error response. Original error: ")
					return errors.Join(errs, egErr)
				}

				switch directorApperrors.ErrorType(errorCode) {
				case directorApperrors.NotFound:
					return err
				default:
					break
				}
			default:
				break
			}
		}
		for _, l := range c.logs {
			if l != "" {
				logrus.Info(l)
			}
		}
	}
	return err
}

func (c *client) addLog(log string) {
	if !c.logging {
		return
	}

	c.logs = append(c.logs, log)
}

func (c *client) clearLogs() {
	c.logs = []string{}
}
