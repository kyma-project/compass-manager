package controllers

import (
	"context"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
)

//go:generate mockery --name=Registrator
type Registrator interface {
	Register(nameFromKymaCR string) (string, error)
	ConfigureRuntimeAgent(kubeconfigSecretName string) error
}

type CompassRegistrator struct{}

func (r *CompassRegistrator) ConfigureRuntimeAgent(kubeconfigSecretName string) error {
	return nil
}

func (r *CompassRegistrator) Register(nameFromKymaCR string) (string, error) {
	return "", nil
}

// CompassManagerReconciler reconciles a CompassManager object
type CompassManagerReconciler struct {
	client.Client
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

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;

var ommitStatusChanged = predicate.Or(
	predicate.GenerationChangedPredicate{},
	predicate.LabelChangedPredicate{},
	predicate.AnnotationChangedPredicate{},
)

func (cm *CompassManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// if call to Director will not succeed trigger call once again, and if failed again wait 2 minutes and repeat whole process

	cm.Log.Infof("reconciliation triggered for resource named: %s", req.Name)
	kymaName := req.Name
	kubeconfigSecretName := "kubeconfig-" + req.Name

	_, err := cm.Registrator.Register(kymaName)
	if err != nil {
		return ctrl.Result{}, err
	}
	cm.Log.Info("Registered")
	err = cm.Registrator.ConfigureRuntimeAgent(kubeconfigSecretName)
	if err != nil {
		return ctrl.Result{}, err
	}
	cm.Log.Info("CRA configured")
	return ctrl.Result{}, nil
}

func (cm *CompassManagerReconciler) checkKubeconfigStrategy(obj runtime.Object) bool {
	kymaObj, ok := obj.(*kyma.Kyma)
	if !ok {
		cm.Log.Errorf("%s", "cannot parse Kyma Custom Resource")
		return false
	}

	if kymaObj.Spec.Sync.Strategy == "" {
		cm.Log.Infof("%s", "Kubeconfig not providied in Kyma Custom Resource")
		return false
	}

	return true
}

func (cm *CompassManagerReconciler) checkUpdateKubeconfigStrategy(objNew, objOld runtime.Object) bool {
	kymaObjNew, okNew := objNew.(*kyma.Kyma)
	kymaObjOld, okOld := objOld.(*kyma.Kyma)

	if !okNew || !okOld {
		cm.Log.Errorf("%s", "cannot parse Kyma Custom Resource")
		return false
	}

	if kymaObjNew.Spec.Sync.Strategy == "" {
		cm.Log.Infof("%s", "Kubeconfig not providied in Kyma Custom Resource")
		return false
	}

	if strings.Compare(string(kymaObjNew.Spec.Sync.Strategy), string(kymaObjOld.Spec.Sync.Strategy)) == 0 {
		cm.Log.Infof("%s", "Kubeconfig has not changed, skipping reconcilation")
		return false
	}
	return true
}

// We can provide some logic to check if user updates the Kubeconfig e.g. field strategy -> check if secret with Kubeconfig is present in kcp-system?

// SetupWithManager sets up the controller with the Manager.
func (cm *CompassManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fieldSelectorPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return cm.checkKubeconfigStrategy(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return cm.checkUpdateKubeconfigStrategy(e.ObjectNew, e.ObjectOld)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return cm.checkKubeconfigStrategy(e.Object)
		},
		DeleteFunc: nil,
	}

	// From documentation
	/*
		// For defines the type of Object being *reconciled*, and configures the ControllerManagedBy to respond to create / delete /
		// update events by *reconciling the object*.
		// This is the equivalent of calling
		// Watches(&source.Kind{Type: apiType}, &handler.EnqueueRequestForObject{}).
	*/
	runner := ctrl.NewControllerManagedBy(mgr).
		For(&kyma.Kyma{}, builder.WithPredicates(
			predicate.And(
				predicate.ResourceVersionChangedPredicate{},
				ommitStatusChanged,
			)))

	return runner.WithEventFilter(fieldSelectorPredicate).Complete(cm)
}
