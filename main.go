package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kyma-project/compass-manager/api/v1beta1"
	"github.com/kyma-project/compass-manager/controllers"
	"github.com/kyma-project/compass-manager/controllers/metrics"
	"github.com/kyma-project/compass-manager/internal/director"
	"github.com/kyma-project/compass-manager/internal/graphql"
	"github.com/kyma-project/compass-manager/internal/oauth"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vrischmann/envconfig"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme   = runtime.NewScheme()        //nolint:gochecknoglobals
	setupLog = ctrl.Log.WithName("setup") //nolint:gochecknoglobals
)

type config struct {
	Address                      string `envconfig:"default=127.0.0.1:3000"`
	APIEndpoint                  string `envconfig:"default=/graphql"`
	PlaygroundAPIEndpoint        string `envconfig:"default=/graphql"`
	SkipDirectorCertVerification bool   `envconfig:"default=false"`
	DirectorURL                  string `envconfig:"APP_DIRECTOR_URL,default=https://compass-gateway-auth-oauth.cmp-main.dev.kyma.cloud.sap/director/graphql"`
	DirectorOAuthPath            string `envconfig:"APP_DIRECTOR_OAUTH_PATH,default=./dev/director.yaml"`
	EnabledRegistration          bool   `envconfig:"APP_ENABLED_REGISTRATION,default=false"`
}

func (c *config) String() string {
	return fmt.Sprintf("Address: %s, APIEndpoint: %s, DirectorURL: %s, SkipDirectorCertVerification: %v, DirectorOAuthPath: %s",
		c.Address, c.APIEndpoint, c.DirectorURL,
		c.SkipDirectorCertVerification, c.DirectorOAuthPath)
}

type DirectorOAuth struct {
	Data struct {
		ClientID       string `json:"client_id"`
		ClientSecret   string `json:"client_secret"`
		TokensEndpoint string `json:"tokens_endpoint"`
	} `json:"data"`
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kyma.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	cfg := config{}
	err := envconfig.InitWithPrefix(&cfg, "APP")
	exitOnError(err, "Failed to load application config")

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "2647ec81.kyma-project.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	directorClient, err := newDirectorClient(cfg)
	if err != nil {
		setupLog.Error(err, "unable to create Director Client")
		os.Exit(1)
	}

	compassRegistrator := controllers.NewCompassRegistrator(directorClient, log)
	runtimeAgentConfigurator := controllers.NewRuntimeAgentConfigurator(directorClient, log)
	requeueTime := time.Second * 5 //nolint:gomnd

	metrics := metrics.NewMetrics()

	compassManagerReconciler := controllers.NewCompassManagerReconciler(
		mgr,
		log,
		runtimeAgentConfigurator,
		compassRegistrator,
		requeueTime,
		cfg.EnabledRegistration,
		metrics,
	)
	if err = compassManagerReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CompassManager")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func newDirectorClient(config config) (director.Client, error) {
	file, err := os.ReadFile(config.DirectorOAuthPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open director config")
	}

	cfg := DirectorOAuth{}
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal director config")
	}

	gqlClient := graphql.NewGraphQLClient(config.DirectorURL, true, config.SkipDirectorCertVerification)
	oauthClient := oauth.NewOauthClient(newHTTPClient(config.SkipDirectorCertVerification), cfg.Data.ClientID, cfg.Data.ClientSecret, cfg.Data.TokensEndpoint)

	return director.NewDirectorClient(gqlClient, oauthClient), nil
}

func newHTTPClient(skipCertVerification bool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipCertVerification},
		},
		Timeout: 30 * time.Second, //nolint:gomnd
	}
}

func exitOnError(err error, context string) {
	if err != nil {
		wrappedError := errors.Wrap(err, context)
		log.Fatal(wrappedError)
	}
}
