package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	log "github.com/sirupsen/logrus"
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
	KymaNameLabel  = "operator.kyma-project.io/kyma-name"
	CompassIDLabel = "operator.kyma-project.io/compass-id"
	// KubeconfigKey is the name of the key in the secret storing cluster credentials.
	// The secret is created by KEB: https://github.com/kyma-project/control-plane/blob/main/components/kyma-environment-broker/internal/process/steps/lifecycle_manager_kubeconfig.go
	KubeconfigKey = "config"
)

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

//go:generate mockery --name=Registrator
type Registrator interface {
	// Register creates Runtime in the Compass system. It must be idempotent.
	Register(name string) (string, error)
	// ConfigureRuntimeAgent creates a config map in the Runtime that is used by the Compass Runtime Agent. It must be idempotent.
	ConfigureRuntimeAgent(kubeconfig string, runtimeID string) error
}

type Client interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error
}

// CompassManagerReconciler reconciles a CompassManager object
type CompassManagerReconciler struct {
	Client      Client
	Scheme      *runtime.Scheme
	Log         *log.Logger
	Registrator Registrator
}

func NewCompassManagerReconciler(mgr manager.Manager, log *log.Logger, r Registrator) *CompassManagerReconciler {
	return &CompassManagerReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Log:         log,
		Registrator: r,
	}
}

var requeueTime = time.Minute * 5

func (cm *CompassManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	cm.Log.Infof("Reconciliation triggered for Kyma Resource %s", req.Name)
	kubeconfig, err := cm.getKubeconfig(req.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if kubeconfig == "" {
		cm.Log.Infof("Kubeconfig for Kyma resource %s not available.", req.Name)
		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}

	compassRuntimeID, err := cm.Registrator.Register(req.Name)
	if err != nil {
		cm.Log.Warnf("Failed to register Runtime for Kyma resource %s: %v.", req.Name, err)
		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}
	cm.Log.Infof("Runtime %s registered for Kyma resource %s.", compassRuntimeID, req.Name)

	err = cm.Registrator.ConfigureRuntimeAgent(kubeconfig, compassRuntimeID)
	if err != nil {
		cm.Log.Warnf("Failed to configure Compass Runtime Agent for Kyma resource %s: %v.", req.Name, err)
		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}
	cm.Log.Infof("Compass Runtime Agent for Runtime %s configured.", compassRuntimeID)

	return ctrl.Result{}, cm.markRuntimeRegistered(req.NamespacedName, compassRuntimeID)
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

func (cm *CompassManagerReconciler) markRuntimeRegistered(objKey types.NamespacedName, compassID string) error {

	instance := &kyma.Kyma{}
	err := cm.Client.Get(context.Background(), objKey, instance)
	if err != nil {
		return err
	}

	l := instance.GetLabels()
	if l == nil {
		l = make(map[string]string)
	}

	l[CompassIDLabel] = compassID

	instance.SetLabels(l)

	return cm.Client.Update(context.Background(), instance)
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
	_, labelFound := kymaObj.GetLabels()[CompassIDLabel]

	return !labelFound
}
