package controllers

import (
	"context"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

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
	return "compass-id", nil
}

//go:generate mockery --name=Client
type Client interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
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

	clusterSecret := &corev1.Secret{}
	clusterSecretName := req.NamespacedName
	clusterSecretName.Name = kubeconfigSecretName

	err := cm.Client.Get(ctx, clusterSecretName, clusterSecret)
	if err != nil {
		cm.Log.Infof("cannot retrieve the Kubeconfig secret associated with Kyma CR named: %s, retying in 10 seconds", kymaName)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	compassId, err := cm.Registrator.Register(kymaName)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
	cm.Log.Info("Registered")
	err = cm.Registrator.ConfigureRuntimeAgent(kubeconfigSecretName)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
	cm.Log.Info("CRA configured")

	cm.applyLabelOnKymaResource(req.NamespacedName, compassId)
	return ctrl.Result{}, nil
}

func (cm *CompassManagerReconciler) checkCompassLabel(obj runtime.Object) bool {
	kymaObj, ok := obj.(*kyma.Kyma)

	if !ok {
		cm.Log.Errorf("%s", "cannot parse Kyma Custom Resource")
		return false
	}
	labels := kymaObj.GetLabels()

	if _, ok := labels["operator.kyma-project.io/compass-id"]; ok {
		cm.Log.Infof("Compass id is present on Kyma Custom Resource named: %s, skipping reconciliation", kymaObj.Name)
		return false
	}

	return true
}

func (cm *CompassManagerReconciler) checkUpdateCompassLabel(objNew, objOld runtime.Object) bool {
	kymaObjNew, okNew := objNew.(*kyma.Kyma)
	kymaObjOld, okOld := objOld.(*kyma.Kyma)

	if !okNew || !okOld {
		cm.Log.Errorf("%s", "cannot parse Kyma Custom Resource")
		return false
	}

	labelsNew := kymaObjNew.GetLabels()
	labelsOld := kymaObjOld.GetLabels()

	if labelsOld["operator.kyma-project.io/compass-id"] == labelsNew["operator.kyma-project.io/compass-id"] {
		cm.Log.Infof("Compass id is present on Kyma Custom Resource named: %s, skipping reconciliation", kymaObjNew.Name)
		return false
	}

	if _, ok := labelsOld["operator.kyma-project.io/compass-id"]; !ok {
		if _, ok := labelsNew["operator.kyma-project.io/compass-id"]; ok {
			cm.Log.Infof("Kyma Custom Resource named: %s successfully labeled with Compass ID", kymaObjNew.Name)
			return false
		}
	}

	//prevent user from deletion of compass-id label. Risk -> if a user edits the label, we will not be able to force a reconciliation by removing the label. We can implement logic that prevents user from updating the label
	if _, ok := labelsOld["operator.kyma-project.io/compass-id"]; ok {
		if _, ok := labelsNew["operator.kyma-project.io/compass-id"]; !ok {
			key := types.NamespacedName{
				Namespace: kymaObjNew.Namespace,
				Name:      kymaObjNew.Name,
			}
			cm.applyLabelOnKymaResource(key, labelsOld["operator.kyma-project.io/compass-id"])
			cm.Log.Infof("user cannot delete compass-id label from Kyma Custom Resource named: %s, reverting changes", kymaObjNew.Name)
			return false
		}
	}

	return true
}

// SetupWithManager sets up the controller with the Manager.
func (cm *CompassManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fieldSelectorPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return cm.checkCompassLabel(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return cm.checkUpdateCompassLabel(e.ObjectNew, e.ObjectOld)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return cm.checkCompassLabel(e.Object)
		},
		DeleteFunc: nil,
	}

	runner := ctrl.NewControllerManagedBy(mgr).
		For(&kyma.Kyma{}, builder.WithPredicates(
			predicate.And(
				predicate.ResourceVersionChangedPredicate{},
				ommitStatusChanged,
			)))

	return runner.WithEventFilter(fieldSelectorPredicate).Complete(cm)
}

func (cm *CompassManagerReconciler) applyLabelOnKymaResource(objKey types.NamespacedName, compassId string) {
	instance := &kyma.Kyma{}
	err := cm.Client.Get(context.Background(), objKey, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kyma Custom Resource not found")
		}
		log.Info("failed to read Kyma Custom Resource")
	}

	l := instance.GetLabels()
	if l == nil {
		l = make(map[string]string)
	}

	l["operator.kyma-project.io/compass-id"] = compassId
	instance.SetLabels(l)

	err = cm.Client.Update(context.Background(), instance)
	if err != nil {
		log.Infof("%v, %s", err, " failed to update Kyma Custom Resource")
	}
}
