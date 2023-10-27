package controllers

import (
	"context"
	"fmt"
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
	AnnotationIDForMigration = "compass-runtime-id-for-migration"

	LabelBrokerInstanceID = "kyma-project.io/instance-id"
	LabelBrokerPlanID     = "kyma-project.io/broker-plan-id"
	LabelBrokerPlanName   = "kyma-project.io/broker-plan-name"
	LabelComppassID       = "kyma-project.io/compass-runtime-id"
	LabelGlobalAccountID  = "kyma-project.io/global-account-id"
	LabelKymaName         = "operator.kyma-project.io/kyma-name"
	LabelManagedBy        = "operator.kyma-project.io/managed-by"
	LabelShootName        = "kyma-project.io/shoot-name"
	LabelSubaccountID     = "kyma-project.io/subaccount-id"

	ApplicationConnectorModuleName = "applicationconnector"
	// KubeconfigKey is the name of the key in the secret storing cluster credentials.
	// The secret is created by KEB: https://github.com/kyma-project/control-plane/blob/main/components/kyma-environment-broker/internal/process/steps/lifecycle_manager_kubeconfig.go
	KubeconfigKey = "config"
)

type DirectorError struct {
	message error
}

func (e *DirectorError) Error() string {
	return fmt.Sprintf("error from director: %s", e.message)
}

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagermappings,verbs=create;get;list;delete;watch;update
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
	// DeregisterFromCompass deletes Runtime from Compass system
	DeregisterFromCompass(compassID, globalAccount string) error
}

type Client interface {
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error
	Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
	Status() client.SubResourceWriter
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

func (cm *CompassManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { // nolint:revive
	cm.Log.Infof("Reconciliation triggered for Kyma Resource %s", req.Name)

	kymaCR, err := cm.getKymaCR(req.NamespacedName)

	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to obtain labels from Kyma resource %s", req.Name)
		}

		delErr := cm.handleKymaDeletion(req.Name, req.Namespace)
		var directorError *DirectorError
		if errors.As(delErr, &directorError) {
			return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
		}

		if delErr != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to perform unregistration stage for Kyma %s", req.Name)
		}
		return ctrl.Result{}, nil
	}

	kubeconfig, err := cm.getKubeconfig(req.Name)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get Kubeconfig object for Kyma: %s", req.Name)
	}

	if kubeconfig == "" {
		cm.Log.Infof("Kubeconfig for Kyma resource %s not available.", req.Name)
		return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
	}

	compassRuntimeID, err := cm.getRuntimeIDFromCompassMapping(req.Name, req.Namespace)

	kymaLabels := kymaCR.Labels
	kymaAnnotations := kymaCR.Annotations

	if migrationCompassRuntimeID, ok := kymaAnnotations[AnnotationIDForMigration]; compassRuntimeID == "" && ok {
		cm.Log.Infof("Configuring compass for already registered Kyma resource %s.", req.Name)
		mapping, cmerr := cm.upsertCompassMappingResource(migrationCompassRuntimeID, req.Namespace, kymaLabels)
		if cmerr != nil {
			return ctrl.Result{RequeueAfter: cm.requeueTime}, errors.Wrap(cmerr, "failed to create Compass Manager Mapping for an already registered Kyma")
		}
		cm.setCompassMappingStatus("", "", true, true, mapping)
		return ctrl.Result{}, nil
	}

	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to obtain Compass Runtime ID from Kyma resource %s", req.Name)
	}

	if compassRuntimeID == "" {
		newCompassRuntimeID, regErr := cm.Registrator.RegisterInCompass(createCompassRuntimeLabels(kymaLabels))
		if regErr != nil {
			_, cmerr := cm.upsertCompassMappingResource("", req.Namespace, kymaLabels)
			if cmerr != nil {
				return ctrl.Result{RequeueAfter: cm.requeueTime}, errors.Wrapf(cmerr, "failed to create Compass Manager Mapping after failed attempt to register runtime for Kyma resource: %s: %v", req.Name, regErr)
			}

			cm.setCompassMappingStatus(req.Namespace, req.Name, false, false, nil)
			cm.Log.Warnf("compass manager mapping created after failed attempt to register runtime for Kyma resource: %s: %v", req.Name, regErr)
			return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
		}

		_, cmerr := cm.upsertCompassMappingResource(newCompassRuntimeID, req.Namespace, kymaLabels)
		if cmerr != nil {
			return ctrl.Result{RequeueAfter: cm.requeueTime}, errors.Wrap(cmerr, "failed to create Compass Manager Mapping after successful attempt to register runtime")
		}

		cm.setCompassMappingStatus(req.Namespace, req.Name, true, false, nil)

		compassRuntimeID = newCompassRuntimeID
		cm.Log.Infof("Runtime %s registered for Kyma resource %s.", newCompassRuntimeID, req.Name)
	}

	err = cm.Configurator.ConfigureCompassRuntimeAgent(kubeconfig, compassRuntimeID)
	if err != nil {
		cm.setCompassMappingStatus(req.Namespace, req.Name, true, false, nil)
		cm.Log.Warnf("Failed to configure Compass Runtime Agent for Kyma resource %s: %v.", req.Name, err)
		return ctrl.Result{RequeueAfter: cm.requeueTime}, err
	}

	cm.setCompassMappingStatus(req.Namespace, req.Name, true, true, nil)
	cm.Log.Infof("Compass Runtime Agent for Runtime %s configured.", compassRuntimeID)

	return ctrl.Result{}, nil
}

func (cm *CompassManagerReconciler) getKubeconfig(kymaName string) (string, error) {
	secretList := &corev1.SecretList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		LabelKymaName: kymaName,
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

func (cm *CompassManagerReconciler) getKymaCR(objKey types.NamespacedName) (kyma.Kyma, error) {
	instance := kyma.Kyma{}

	err := cm.Client.Get(context.Background(), objKey, &instance)
	if err != nil {
		return instance, err
	}

	if instance.Labels == nil {
		instance.Labels = make(map[string]string)
	}
	if instance.Annotations == nil {
		instance.Annotations = make(map[string]string)
	}

	return instance, nil
}

func (cm *CompassManagerReconciler) upsertCompassMappingResource(compassRuntimeID, namespace string, kymaLabels map[string]string) (*v1beta1.CompassManagerMapping, error) {
	kymaName := kymaLabels[LabelKymaName]
	compassMapping := &v1beta1.CompassManagerMapping{}
	compassMapping.Name = kymaName
	compassMapping.Namespace = namespace
	compassMapping.Status.Registered = true

	compassMappingLabels := make(map[string]string)
	compassMappingLabels[LabelKymaName] = kymaLabels[LabelKymaName]
	compassMappingLabels[LabelComppassID] = compassRuntimeID
	compassMappingLabels[LabelGlobalAccountID] = kymaLabels[LabelGlobalAccountID]
	compassMappingLabels[LabelSubaccountID] = kymaLabels[LabelSubaccountID]
	compassMappingLabels[LabelManagedBy] = "compass-manager"

	compassMapping.SetLabels(compassMappingLabels)

	key := types.NamespacedName{
		Name:      kymaName,
		Namespace: namespace,
	}

	existingMapping := v1beta1.CompassManagerMapping{}
	err := cm.Client.Get(context.TODO(), key, &existingMapping)
	if k8serrors.IsNotFound(err) {
		return compassMapping, cm.Client.Create(context.Background(), compassMapping)
	}

	existingMapping.SetLabels(compassMappingLabels)
	return compassMapping, cm.Client.Update(context.TODO(), &existingMapping)
}

func (cm *CompassManagerReconciler) getRuntimeIDFromCompassMapping(kymaName, namespace string) (string, error) {
	mappingList := &v1beta1.CompassManagerMappingList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		LabelKymaName: kymaName,
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

	return mappingList.Items[0].GetLabels()[LabelComppassID], nil
}

// setCompassMappingStatus sets the status of specified compass mapping.
// If `existingMapping` is non-nil, it ignores namespace and kymaName and uses provided mapping
// Otherwise it tries to fetch the mapping based on `namespace` and `kymaName`
func (cm *CompassManagerReconciler) setCompassMappingStatus(namespace, kymaName string, registered, configured bool, existingMapping *v1beta1.CompassManagerMapping) {
	if existingMapping == nil {
		mappingList := v1beta1.CompassManagerMappingList{}
		labelSelector := labels.SelectorFromSet(map[string]string{
			LabelKymaName: kymaName,
		})

		err := cm.Client.List(context.Background(), &mappingList, &client.ListOptions{
			LabelSelector: labelSelector,
			Namespace:     namespace,
		})
		if err != nil {
			cm.Log.Warnf("Tried to update Compass Mapping Status, but couldn't read the resource %s: %v", kymaName, err)
			return
		}

		if len(mappingList.Items) == 0 {
			cm.Log.Warnf("Tried to update Compass Mapping Status, but couldn't - the mapping doesn't exist for %s", kymaName)
			return
		}

		existingMapping = &mappingList.Items[0]
	}
	existingMapping.Status.Registered = registered
	existingMapping.Status.Configured = configured
	err := cm.Client.Status().Update(context.TODO(), existingMapping)
	if err != nil {
		cm.Log.Warnf("Failed to update Compass Mapping Status for %s: %v", kymaName, err)
	} else {
		cm.Log.Debugf("Updated Compass Mapping Status for %s to r%v c%v", kymaName, registered, configured)
	}
}

func (cm *CompassManagerReconciler) getGlobalAccountFromCompassMapping(kymaName, namespace string) (string, error) {
	mappingList := &v1beta1.CompassManagerMappingList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		LabelKymaName: kymaName,
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

	return mappingList.Items[0].GetLabels()[LabelGlobalAccountID], nil
}

func (cm *CompassManagerReconciler) handleKymaDeletion(kymaName, namespace string) error {
	runtimeIDFromMapping, err := cm.getRuntimeIDFromCompassMapping(kymaName, namespace)
	if runtimeIDFromMapping == "" {
		cm.Log.Infof("Runtime was not connected in Compass, nothing to delete")
		return nil
	}
	if err != nil {
		cm.Log.Warnf("Failed to obtain labels from Compass Mapping %s: %v", kymaName, err)
		return err
	}

	globalAccountFromMapping, err := cm.getGlobalAccountFromCompassMapping(kymaName, namespace)
	if err != nil {
		cm.Log.Warnf("Failed to obtain labels from Compass Mapping %s: %v", kymaName, err)
		return err
	}

	cm.Log.Infof("Runtime deregistration in Compass for Kyma Resource %s", kymaName)
	err = cm.Registrator.DeregisterFromCompass(runtimeIDFromMapping, globalAccountFromMapping)
	if err != nil {
		cm.Log.Warnf("Failed to deregister Runtime from Compass for Kyma Resource %s: %v", kymaName, err)
		return errors.Wrap(&DirectorError{message: err}, "failed to deregister Runtime from Compass")
	}

	err = cm.deleteCompassMapping(kymaName, namespace)
	if err != nil {
		return errors.Wrap(err, "failed to delete Compass Mapping")
	}
	cm.Log.Infof("Runtime %s deregistered from Compass", kymaName)
	return nil
}

func (cm *CompassManagerReconciler) deleteCompassMapping(kymaName, namespace string) error {
	mapping := &v1beta1.CompassManagerMapping{}

	key := types.NamespacedName{
		Name:      kymaName,
		Namespace: namespace,
	}

	// TODOs add retry for delete logic
	err := cm.Client.Get(context.Background(), key, mapping)
	if err != nil {
		return err
	}

	err = cm.Client.Delete(context.Background(), mapping)
	if err != nil {
		return err
	}

	return nil
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
		DeleteFunc: func(e event.DeleteEvent) bool {
			return cm.needsToBeDeleted(e.Object)
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
		if v.Name == ApplicationConnectorModuleName {
			return true
		}
	}

	return false
}

func (cm *CompassManagerReconciler) needsToBeDeleted(obj runtime.Object) bool {
	_, ok := obj.(*kyma.Kyma)
	if !ok {
		cm.Log.Error("Unexpected type detected. Object type is supposed to be of Kyma type.")
		return false
	}

	return true
}

func createCompassRuntimeLabels(kymaLabels map[string]string) map[string]interface{} {
	runtimeLabels := make(map[string]interface{})
	runtimeLabels["director_connection_managed_by"] = "compass-manager"
	runtimeLabels["broker_instance_id"] = kymaLabels[LabelBrokerInstanceID]
	runtimeLabels["gardenerClusterName"] = kymaLabels[LabelShootName]
	runtimeLabels["subaccount_id"] = kymaLabels[LabelSubaccountID]
	runtimeLabels["global_account_id"] = kymaLabels[LabelGlobalAccountID]
	runtimeLabels["broker_plan_id"] = kymaLabels[LabelBrokerPlanID]
	runtimeLabels["broker_plan_name"] = kymaLabels[LabelBrokerPlanName]

	return runtimeLabels
}
