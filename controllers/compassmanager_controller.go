package controllers

import (
	"context"
	"fmt"
	"time"

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
	ManagedBy = "compass-manager"

	AnnotationIDForMigration = "compass-runtime-id-for-migration"

	Finalizer             = "kyma-project.io/cm-protection"
	LabelBrokerInstanceID = "kyma-project.io/instance-id"
	LabelBrokerPlanID     = "kyma-project.io/broker-plan-id"
	LabelBrokerPlanName   = "kyma-project.io/broker-plan-name"
	LabelCompassID        = "kyma-project.io/compass-runtime-id"
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

var errNotFound = errors.New("resource not found")

type DirectorError struct {
	message error
}

func (e *DirectorError) Error() string {
	return fmt.Sprintf("error from director: %s", e.message)
}

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagermappings,verbs=create;get;list;delete;watch;update
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagermappings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagermappings/finalizers,verbs=update;get
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

//go:generate mockery --name=Configurator
type Configurator interface {
	// ConfigureCompassRuntimeAgent creates a secret in the Runtime that is used by the Compass Runtime Agent. It must be idempotent.
	ConfigureCompassRuntimeAgent(kubeconfig []byte, compassRuntimeID, globalAccount string) error
}

//go:generate mockery --name=Registrator
type Registrator interface {
	// RegisterInCompass creates Runtime in the Compass system. It must be idempotent.
	RegisterInCompass(compassRuntimeLabels map[string]interface{}) (string, error)
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
	Client              Client
	Scheme              *runtime.Scheme
	Log                 *log.Logger
	Configurator        Configurator
	Registrator         Registrator
	requeueTime         time.Duration
	enabledRegistration bool
	cluster             *ControlPlaneInterface
}

func NewCompassManagerReconciler(mgr manager.Manager, log *log.Logger, c Configurator, r Registrator, requeueTime time.Duration, enabledRegistration bool) *CompassManagerReconciler {
	return &CompassManagerReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		Log:                 log,
		Configurator:        c,
		Registrator:         r,
		requeueTime:         requeueTime,
		enabledRegistration: enabledRegistration,
		cluster:             NewControlPlaneInterface(mgr.GetClient(), log),
	}
}

func (cm *CompassManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { // nolint:revive
	cm.Log.Infof("Reconciliation triggered for Kyma Resource %s", req.Name)

	kymaCR, err := cm.cluster.GetKyma(req.NamespacedName)

	// KymaCR doesn't exist - reconcile was triggered by deletion
	if isNotFound(err) {
		delErr := cm.handleKymaDeletion(req.NamespacedName)
		var directorError *DirectorError
		if errors.As(delErr, &directorError) {
			return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
		}

		if delErr != nil {
			return ctrl.Result{}, errors.Wrapf(delErr, "failed to perform unregistration stage for Kyma %s", req.Name)
		}
		return ctrl.Result{}, nil
	}

	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to obtain Kyma resource %s", req.Name)
	}

	// KymaCR exists, get its kubeconfig
	kubeconfig, err := cm.cluster.GetKubeconfig(req.NamespacedName)

	// Kubeconfig doesn't exist / is empty
	if isNotFound(err) || len(kubeconfig) == 0 {
		cm.Log.Infof("Kubeconfig for Kyma resource %s not available.", req.Name)
		return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
	}

	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get Kubeconfig object for Kyma: %s", req.Name)
	}

	// Kyma exists and has a kubeconfig, get the compass mapping
	compassRuntimeID, runtimeIDErr := cm.cluster.GetCompassRuntimeID(req.NamespacedName)

	if runtimeIDErr != nil && !isNotFound(runtimeIDErr) {
		return ctrl.Result{}, errors.Wrapf(runtimeIDErr, "failed to obtain Compass Mapping for Kyma resource %s", req.Name)
	}

	globalAccount, ok := kymaCR.Labels[LabelGlobalAccountID]
	if !ok {
		return ctrl.Result{}, errors.Wrap(err, "failed to obtain Global Account label from Kyma CR")
	}

	/// Part 1 - If compass mapping doesn't exist let's create it and requeue
	if isNotFound(runtimeIDErr) {
		return cm.makeNewCompassMappingAndRequeue(req.NamespacedName, kymaCR.Annotations)
	}

	mapping, err := cm.cluster.GetCompassMapping(req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to obtain Compass Manager Mapping for status checks")
	}
	status := statusNumber(mapping.Status)

	if mapping.Status.State == "" || status&(Failed) == 1 {
		return cm.setStatusAndRequeue(req.NamespacedName, Processing)
	}

	// From this point we will always deal with Compass Manager Mapping for KymaCR
	// Part 2 - If compass mapping doesn't contain valid runtime ID - register runtime and requeue
	if len(compassRuntimeID) == 0 && cm.enabledRegistration {
		return cm.registerRuntimeInCompassAndRequeue(req.NamespacedName, kymaCR.Labels)
	}

	// Uncomment it if you want explicitly have "Registered|Processing" state for a while
	//if status&(Registered|Processing) == 0 {
	//	return cm.setStatusAndRequeue(req.NamespacedName, Registered|Processing)
	//}

	// From that moment we will always deal with Compass Manager Mapping with ID of registered Runtime, or feature flag is disabled
	return cm.configureRuntimeAndSetMappingStatus(req.NamespacedName, kubeconfig, compassRuntimeID, globalAccount)
}

func (cm *CompassManagerReconciler) handleKymaDeletion(name types.NamespacedName) error {
	compass, err := cm.cluster.GetCompassMapping(name)

	if isNotFound(err) {
		cm.Log.Warnf("Runtime %s has no compass mapping, nothing to delete", name)
		return nil
	}

	if err != nil {
		cm.Log.Warnf("Failed to obtain Compass Mapping for Kyma %s: %v", name.Name, err)
		return err
	}

	runtimeIDFromMapping, ok := compass.Labels[LabelCompassID]

	if !ok || runtimeIDFromMapping == "" {
		cm.Log.Infof("Runtime was not connected in Compass, nothing to delete")
		return nil
	}

	globalAccountFromMapping, ok := compass.Labels[LabelGlobalAccountID]
	if !ok {
		cm.Log.Warnf("Compass Mapping for %s has no Global Account", name.Name)
		return errors.Errorf("Compass Mapping for %s has no Global Account", name.Name)
	}

	cm.Log.Infof("Runtime deregistration in Compass for Kyma Resource %s", name.Name)
	err = cm.Registrator.DeregisterFromCompass(runtimeIDFromMapping, globalAccountFromMapping)
	if err != nil {
		cm.Log.Warnf("Failed to deregister Runtime from Compass for Kyma Resource %s: %v", name.Name, err)
		return errors.Wrap(&DirectorError{message: err}, "failed to deregister Runtime from Compass")
	}

	err = cm.cluster.DeleteCompassMapping(name)
	if err != nil {
		return errors.Wrap(err, "failed to delete Compass Mapping")
	}
	cm.Log.Infof("Runtime %s deregistered from Compass", name.Name)
	return nil
}

func (cm *CompassManagerReconciler) makeNewCompassMappingAndRequeue(kymaName types.NamespacedName, kymaAnnotations map[string]string) (ctrl.Result, error) {
	var runtimeID string

	// default mode - application-connector module is enabled for the first time in Kyma, we create Compass Manager Mapping
	runtimeRegistrationType := "newly provisioned Kyma runtime"

	// remove below after migration is completed
	if migrationCompassRuntimeID, ok := kymaAnnotations[AnnotationIDForMigration]; ok && len(migrationCompassRuntimeID) > 0 {
		// Runtime registered previously by Provisioner, but we have the Compass ID provided by KEB
		runtimeRegistrationType = "already registered Kyma runtime"
		runtimeID = migrationCompassRuntimeID
	}

	cm.Log.Infof("Attempting to create Compass Manager Mapping for %s for Kyma resource %s.", runtimeRegistrationType, kymaName.Name)
	cmerr := cm.cluster.CreateCompassMapping(kymaName, runtimeID)
	if cmerr != nil {
		return ctrl.Result{Requeue: true}, errors.Wrapf(cmerr, "failed to create Compass Manager Mapping for %s for Kyma resource ID %s", runtimeRegistrationType, kymaName.Name)
	}
	return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
}

func (cm *CompassManagerReconciler) registerRuntimeInCompassAndRequeue(kymaName types.NamespacedName, kymaLabels map[string]string) (ctrl.Result, error) {
	cm.Log.Infof("Attempting to register runtime in compass for Kyma resource %s.", kymaName.Name)

	newCompassRuntimeID, regError := cm.Registrator.RegisterInCompass(createCompassRuntimeLabels(kymaLabels))

	if regError != nil {
		cm.Log.Errorf("Failed attempt to register runtime for Kyma resource: %s: %v", kymaName.Name, regError)
		statErr := cm.cluster.SetCompassMappingStatus(kymaName, Failed)

		if statErr != nil {
			return ctrl.Result{Requeue: true}, errors.Wrap(statErr, "failed to set Compass Manager Status after failed attempt to register runtime")
		}

		return ctrl.Result{Requeue: true}, errors.Wrapf(regError, "failed attempt to register runtime for Kyma resource: %s", kymaName.Name)
	}

	cm.Log.Infof("Runtime %s registered in Compass", newCompassRuntimeID)
	cmerr := cm.cluster.UpsertCompassMapping(kymaName, newCompassRuntimeID)
	if cmerr != nil {
		return ctrl.Result{Requeue: true}, errors.Wrap(cmerr, "failed to update Compass Manager Mapping with RuntimeID after registration of runtime")
	}

	return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
}

func (cm *CompassManagerReconciler) configureRuntimeAndSetMappingStatus(kymaName types.NamespacedName, kubeconfig []byte, compassRuntimeID, globalAccount string) (ctrl.Result, error) {
	cm.Log.Infof("Attempting to configure Compass Runtime Agent for Runtime %s", compassRuntimeID)

	cfgError := cm.Configurator.ConfigureCompassRuntimeAgent(kubeconfig, compassRuntimeID, globalAccount)
	if cfgError != nil {
		cm.Log.Errorf("Failed attempt to configure Compass Runtime Agent for Kyma resource %s", kymaName.Name)

		statErr := cm.cluster.SetCompassMappingStatus(kymaName, Registered|Failed)
		if statErr != nil {
			return ctrl.Result{Requeue: true}, errors.Wrap(statErr, "failed to set Compass Manager Status after failed attempt configuration Compass Runtime Agent ")
		}

		return ctrl.Result{Requeue: true}, errors.Wrapf(cfgError, "failed attempt to configure Compass Runtime Agent for Kyma resource %s", kymaName.Name)
	}

	cm.Log.Infof("Compass Runtime Agent for Runtime %s configured.", compassRuntimeID)

	statErr := cm.cluster.SetCompassMappingStatus(kymaName, Registered|Configured)
	if statErr != nil {
		return ctrl.Result{Requeue: true}, errors.Wrap(statErr, "failed to set Compass Manager Status after successful configuration Compass Runtime Agent ")
	}

	return ctrl.Result{}, nil
}

func (cm *CompassManagerReconciler) setStatusAndRequeue(kymaName types.NamespacedName, status Status) (ctrl.Result, error) {
	err := cm.cluster.SetCompassMappingStatus(kymaName, status)
	if err != nil {
		return ctrl.Result{Requeue: true}, errors.Wrap(err, "failed to update Compass Manager Mapping status")
	}
	return ctrl.Result{RequeueAfter: cm.requeueTime}, nil
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
	runtimeLabels["director_connection_managed_by"] = ManagedBy
	runtimeLabels["broker_instance_id"] = kymaLabels[LabelBrokerInstanceID]
	runtimeLabels["gardenerClusterName"] = kymaLabels[LabelShootName]
	runtimeLabels["subaccount_id"] = kymaLabels[LabelSubaccountID]
	runtimeLabels["global_account_id"] = kymaLabels[LabelGlobalAccountID]
	runtimeLabels["broker_plan_id"] = kymaLabels[LabelBrokerPlanID]
	runtimeLabels["broker_plan_name"] = kymaLabels[LabelBrokerPlanName]

	return runtimeLabels
}

type ControlPlaneInterface struct {
	log     *log.Logger
	kubectl Client
}

func NewControlPlaneInterface(kubectl Client, log *log.Logger) *ControlPlaneInterface {
	return &ControlPlaneInterface{
		log:     log,
		kubectl: kubectl,
	}
}

func (c *ControlPlaneInterface) GetKyma(name types.NamespacedName) (kyma.Kyma, error) {
	kymaCR := kyma.Kyma{}

	err := c.kubectl.Get(context.TODO(), name, &kymaCR)
	if err != nil {
		return kymaCR, err
	}

	if kymaCR.Labels == nil {
		kymaCR.Labels = make(map[string]string)
	}
	if kymaCR.Annotations == nil {
		kymaCR.Annotations = make(map[string]string)
	}

	return kymaCR, nil
}

func (c *ControlPlaneInterface) GetCompassMapping(name types.NamespacedName) (v1beta1.CompassManagerMapping, error) {
	mapping := v1beta1.CompassManagerMapping{}

	mappingList := &v1beta1.CompassManagerMappingList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		LabelKymaName: name.Name,
	})

	err := c.kubectl.List(context.TODO(), mappingList, &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     name.Namespace,
	})

	if err != nil {
		return mapping, err
	}

	if len(mappingList.Items) == 0 {
		return mapping, errNotFound
	}

	mapping = mappingList.Items[0]

	if mapping.Labels == nil {
		mapping.Labels = make(map[string]string)
	}
	if mapping.Annotations == nil {
		mapping.Annotations = make(map[string]string)
	}

	return mapping, nil
}

func (c *ControlPlaneInterface) DeleteCompassMapping(name types.NamespacedName) error {
	mapping, err := c.GetCompassMapping(name)
	if err != nil {
		return err
	}

	err = c.RemoveCMFinalizer(name)
	if err != nil {
		c.log.Warnf("Couldn't remove finalizer for %s", name)
		return err
	}

	if mapping.DeletionTimestamp != nil {
		return nil
	}

	return c.kubectl.Delete(context.TODO(), &mapping)
}

func (c *ControlPlaneInterface) RemoveCMFinalizer(name types.NamespacedName) error {
	mapping, err := c.GetCompassMapping(name)
	if err != nil {
		return err
	}

	for i, finalizer := range mapping.Finalizers {
		if finalizer == Finalizer {
			mapping.Finalizers = append(mapping.Finalizers[:i], mapping.Finalizers[i+1:]...)
			break
		}
	}

	return c.kubectl.Update(context.TODO(), &mapping)
}

func (c *ControlPlaneInterface) GetKubeconfig(name types.NamespacedName) ([]byte, error) {
	secretList := &corev1.SecretList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		LabelKymaName: name.Name,
	})

	err := c.kubectl.List(context.TODO(), secretList, &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     name.Namespace,
	})

	if err != nil || len(secretList.Items) == 0 {
		return nil, err
	}

	_, ok := secretList.Items[0].Data[KubeconfigKey]
	if !ok {
		return nil, errNotFound
	}

	kubecfg := &secretList.Items[0]

	return kubecfg.Data[KubeconfigKey], nil
}

func (c *ControlPlaneInterface) UpsertCompassMapping(name types.NamespacedName, compassRuntimeID string) error {
	kymaCR, err := c.GetKyma(name)
	if err != nil {
		return err
	}

	labels := make(map[string]string)
	labels[LabelKymaName] = kymaCR.Labels[LabelKymaName]
	labels[LabelCompassID] = compassRuntimeID
	labels[LabelGlobalAccountID] = kymaCR.Labels[LabelGlobalAccountID]
	labels[LabelSubaccountID] = kymaCR.Labels[LabelSubaccountID]
	labels[LabelManagedBy] = ManagedBy

	existingMapping, err := c.GetCompassMapping(name)

	if isNotFound(err) {
		newMapping := &v1beta1.CompassManagerMapping{}
		newMapping.Name = name.Name
		newMapping.Namespace = name.Namespace
		newMapping.Labels = labels
		newMapping.Finalizers = []string{Finalizer}

		cerr := c.kubectl.Create(context.TODO(), newMapping)
		if cerr != nil {
			return cerr
		}
		return nil
	}

	if err != nil {
		return err
	}

	existingMapping.Labels = labels
	err = c.kubectl.Update(context.TODO(), &existingMapping)
	if err != nil {
		return err
	}

	return err
}

func (c *ControlPlaneInterface) CreateCompassMapping(name types.NamespacedName, compassRuntimeID string) error {
	kymaCR, err := c.GetKyma(name)
	if err != nil {
		return err
	}

	labels := make(map[string]string)
	labels[LabelKymaName] = kymaCR.Labels[LabelKymaName]
	labels[LabelCompassID] = compassRuntimeID
	labels[LabelGlobalAccountID] = kymaCR.Labels[LabelGlobalAccountID]
	labels[LabelSubaccountID] = kymaCR.Labels[LabelSubaccountID]
	labels[LabelManagedBy] = ManagedBy

	newMapping := v1beta1.CompassManagerMapping{}
	newMapping.Name = name.Name
	newMapping.Namespace = name.Namespace
	newMapping.Labels = labels
	newMapping.Finalizers = []string{Finalizer}

	err = c.kubectl.Create(context.TODO(), &newMapping)
	return err
}

// GetCompassRuntimeID returns `errNotFound` if the mapping exists, but doesn't have the label
func (c *ControlPlaneInterface) GetCompassRuntimeID(name types.NamespacedName) (string, error) {
	mapping, err := c.GetCompassMapping(name)
	if err != nil {
		return "", err
	}
	return mapping.Labels[LabelCompassID], nil
}

// SetCompassMappingStatus sets the registered and configured on an existing CompassManagerMapping
// If error occurs - logs it and returns
func (c *ControlPlaneInterface) SetCompassMappingStatus(name types.NamespacedName, status Status) error {
	mapping, err := c.GetCompassMapping(name)
	if err != nil {
		return err
	}

	registered := status&Registered != 0
	configured := status&Configured != 0
	state := stateText(status)

	mapping.Status = v1beta1.CompassManagerMappingStatus{
		Registered: registered,
		Configured: configured,
		State:      state,
	}

	err = c.kubectl.Status().Update(context.TODO(), &mapping)
	if err != nil {
		c.log.Warnf("Failed to update Compass Mapping Status for %s: %v", name.Name, err)
	} else {
		c.log.Infof("Updated Compass Mapping Status for %s: registered=%v, configured=%v, state=%s", name.Name, registered, configured, state)
	}
	return err
}

func isNotFound(err error) bool {
	return k8serrors.IsNotFound(err) || errors.Is(err, errNotFound)
}
