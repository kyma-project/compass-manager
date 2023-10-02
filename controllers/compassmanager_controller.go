package controllers

import (
	"context"
	"time"

	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	"github.com/kyma-project/compass-manager/api/v1beta1"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	KymaNameLabel                  = "operator.kyma-project.io/kyma-name"
	BrokerPlanIDLabel              = "kyma-project.io/broker-plan-id"
	BrokerPlanNameLabel            = "kyma-project.io/broker-plan-name"
	GlobalAccountIDLabel           = "kyma-project.io/global-account-id"
	BrokerInstanceIDLabel          = "kyma-project.io/instance-id"
	ShootNameLabel                 = "kyma-project.io/shoot-name"
	SubaccountIDLabel              = "kyma-project.io/subaccount-id"
	ComppassIDLabel                = "kyma-project.io/compass-runtime-id"
	ManagedByLabel                 = "operator.kyma-project.io/managed-by"
	ApplicationConnectorModuleName = "application-connector-module"
	// KubeconfigKey is the name of the key in the secret storing cluster credentials.
	// The secret is created by KEB: https://github.com/kyma-project/control-plane/blob/main/components/kyma-environment-broker/internal/process/steps/lifecycle_manager_kubeconfig.go
	KubeconfigKey = "config"
)

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagermappings,verbs=create;get;list;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

//go:generate mockery --name=Configurator
type Configurator interface {
	// ConfigureCompassRuntimeAgent creates a secret in the Runtime that is used by the Compass Runtime Agent. It must be idempotent.
	ConfigureCompassRuntimeAgent(kubeconfig string, runtimeID string) error
	// UpdateCompassRuntimeAgent updates the secret in the Runtime that is used by the Compass Runtime Agent
	UpdateCompassRuntimeAgent(kubeconfig string) error
}

//go:generate mockery --name=Registrator
type Registrator interface {
	// RegisterInCompass creates Runtime in the Compass system. It must be idempotent.
	RegisterInCompass(compassRuntimeLabels map[string]interface{}) (string, error)
	// RefreshCompassToken gets new connection token for Compass requests
	RefreshCompassToken(compassID, globalAccount string) (graphql.OneTimeTokenForRuntimeExt, error)
}

type Client interface {
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error
}

// CompassManagerReconciler reconciles a CompassManager object
type CompassManagerReconciler struct {
	Client       Client
	Scheme       *runtime.Scheme
	Log          *log.Logger
	Configurator Configurator
	Registrator  Registrator
	requeueTime  time.Duration
}

func NewCompassManagerReconciler(mgr manager.Manager, log *log.Logger, c Configurator, r Registrator, requeueTime time.Duration) *CompassManagerReconciler {
	return &CompassManagerReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Log:          log,
		Configurator: c,
		Registrator:  r,
		requeueTime:  requeueTime,
	}
}

func (cm *CompassManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:revive
	cm.Log.Infof("Reconciliation triggered for Kyma Resource %s", req.Name)
	kubeconfig, err := cm.getKubeconfig(req.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if kubeconfig == "" {
		cm.Log.Infof("Kubeconfig for Kyma resource %s not available.", req.Name)
		return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
	}

	kymaLabels, err := cm.getKymaLabels(req.NamespacedName)
	if err != nil {
		cm.Log.Warnf("Failed to obtain labels from Kyma resource %s: %v.", req.Name, err)
		return ctrl.Result{RequeueAfter: cm.requeueTime}, err
	}

	compassRuntimeID, err := cm.getRuntimeIDFromCompassMapping(req.Name, req.Namespace)
	if err != nil {
		return ctrl.Result{RequeueAfter: cm.requeueTime}, err
	}

	if compassRuntimeID == "" {
		newCompassRuntimeID, regErr := cm.Registrator.RegisterInCompass(createCompassRuntimeLabels(kymaLabels))
		if regErr != nil {
			cmerr := cm.upsertCompassMappingResource("", req.Namespace, kymaLabels)
			if cmerr != nil {
				return ctrl.Result{RequeueAfter: cm.requeueTime}, errors.Wrap(cmerr, "failed to create Compass Manager Mapping after failed attempt to register runtime")
			}

			return ctrl.Result{RequeueAfter: cm.requeueTime}, err
		}

		cmerr := cm.upsertCompassMappingResource(newCompassRuntimeID, req.Namespace, kymaLabels)
		if cmerr != nil {
			return ctrl.Result{RequeueAfter: cm.requeueTime}, errors.Wrap(cmerr, "failed to create Compass Manager Mapping after successful attempt to register runtime")
		}

		compassRuntimeID = newCompassRuntimeID
		cm.Log.Infof("Runtime %s registered for Kyma resource %s.", newCompassRuntimeID, req.Name)
	}

	err = cm.Configurator.ConfigureCompassRuntimeAgent(kubeconfig, compassRuntimeID)
	if err != nil {
		cm.Log.Warnf("Failed to configure Compass Runtime Agent for Kyma resource %s: %v.", req.Name, err)
		return ctrl.Result{RequeueAfter: cm.requeueTime}, err
	}
	cm.Log.Infof("Compass Runtime Agent for Runtime %s configured.", compassRuntimeID)

	return ctrl.Result{}, nil
}

func (cm *CompassManagerReconciler) getKubeconfig(kymaName string) (string, error) {
	secretList := &corev1.SecretList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		KymaNameLabel: kymaName,
	})

	err := cm.Client.List(context.Background(), secretList, &client.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		return "", err
	}

	if len(secretList.Items) == 0 {
		return "", nil
	}
	secret := &secretList.Items[0]

	return string(secret.Data[KubeconfigKey]), nil
}

func (cm *CompassManagerReconciler) getKymaLabels(objKey types.NamespacedName) (map[string]string, error) {
	instance := &kyma.Kyma{}

	err := cm.Client.Get(context.Background(), objKey, instance)
	if err != nil {
		return nil, err
	}

	l := instance.GetLabels()
	if l == nil {
		l = make(map[string]string)
	}

	return l, nil
}

func (cm *CompassManagerReconciler) upsertCompassMappingResource(compassRuntimeID, namespace string, kymaLabels map[string]string) error {
	kymaName := kymaLabels[KymaNameLabel]
	compassMapping := &v1beta1.CompassManagerMapping{}
	compassMapping.Name = kymaName
	compassMapping.Namespace = namespace

	compassMappingLabels := make(map[string]string)
	compassMappingLabels[KymaNameLabel] = kymaLabels[KymaNameLabel]
	compassMappingLabels[ComppassIDLabel] = compassRuntimeID
	compassMappingLabels[GlobalAccountIDLabel] = kymaLabels[GlobalAccountIDLabel]
	compassMappingLabels[SubaccountIDLabel] = kymaLabels[SubaccountIDLabel]
	compassMappingLabels[ManagedByLabel] = "compass-manager"

	compassMapping.SetLabels(compassMappingLabels)

	key := types.NamespacedName{
		Name:      kymaName,
		Namespace: namespace,
	}

	existingMapping := v1beta1.CompassManagerMapping{}
	// TODOs add retry for upsert logic
	err := cm.Client.Get(context.TODO(), key, &existingMapping)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return cm.Client.Create(context.Background(), compassMapping)
		}
	}

	existingMapping.SetLabels(compassMappingLabels)
	return cm.Client.Update(context.TODO(), &existingMapping)
}

func (cm *CompassManagerReconciler) getRuntimeIDFromCompassMapping(kymaName string, namespace string) (string, error) {
	mappingList := &v1beta1.CompassManagerMappingList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		KymaNameLabel: kymaName,
	})

	err := cm.Client.List(context.Background(), mappingList, &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     namespace,
	})

	if err != nil {
		return "", err
	}

	if len(mappingList.Items) == 0 {
		return "", nil
	}

	return mappingList.Items[0].GetLabels()[ComppassIDLabel], nil
}

// SetupWithManager sets up the controller with the Manager.
func (cm *CompassManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fieldSelectorPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return cm.needsToBeReconciled(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return cm.needsToBeReconciled(e.ObjectNew)
		},
	}

	omitStatusChanged := predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.LabelChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)

	// We can simplify passing the predicate filters to controller
	// The predicates passed in For(builder.WithPredicates()) function is merged with runner.WithEventFilter() predicates to single slice with predicates.
	// Proposal: delete the predicates from For() functions, and return runner.WithEventFilter(fieldSelectorPredicate).WithEventFilter(predicates).Complete(cm)

	runner := ctrl.NewControllerManagedBy(mgr).
		For(&kyma.Kyma{}, builder.WithPredicates(
			predicate.And(
				predicate.ResourceVersionChangedPredicate{},
				omitStatusChanged,
			)))

	return runner.WithEventFilter(fieldSelectorPredicate).Complete(cm)
}

func (cm *CompassManagerReconciler) needsToBeReconciled(obj runtime.Object) bool {
	kymaObj, ok := obj.(*kyma.Kyma)
	if !ok {
		cm.Log.Error("Unexpected type detected. Object type is supposed to be of Kyma type.")
		return false
	}

	kymaModules := kymaObj.Spec.Modules

	for _, v := range kymaModules {
		// Placeholder for App Conn module name, change if the name will be already known
		if v.Name == ApplicationConnectorModuleName {
			return true
		}
	}

	return false
}

func createCompassRuntimeLabels(kymaLabels map[string]string) map[string]interface{} {
	runtimeLabels := make(map[string]interface{})
	runtimeLabels["director_connection_managed_by"] = "compass-manager"
	runtimeLabels["broker_instance_id"] = kymaLabels[BrokerInstanceIDLabel]
	runtimeLabels["gardenerClusterName"] = kymaLabels[ShootNameLabel]
	runtimeLabels["subaccount_id"] = kymaLabels[SubaccountIDLabel]
	runtimeLabels["global_account_id"] = kymaLabels[GlobalAccountIDLabel]
	runtimeLabels["broker_plan_id"] = kymaLabels[BrokerPlanIDLabel]
	runtimeLabels["broker_plan_name"] = kymaLabels[BrokerPlanNameLabel]

	return runtimeLabels
}
