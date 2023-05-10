package controllers

import (
	"context"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
)

// CompassManagerReconciler reconciles a CompassManager object
type CompassManagerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      *log.Logger
	KymaObjs []kyma.Kyma
}

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=compassmanagers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CompassManager object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile

var ommitStatusChanged = predicate.Or(
	predicate.GenerationChangedPredicate{},
	predicate.LabelChangedPredicate{},
	predicate.AnnotationChangedPredicate{},
)

func (r *CompassManagerReconciler) mapFunction(object client.Object) []reconcile.Request {
	var kymas kyma.KymaList
	err := r.List(context.Background(), &kymas)

	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		r.Log.Errorf("%v", err)
	}

	if len(kymas.Items) < 1 {
		return nil
	}

	instanceIsBeingDeleted := !kymas.Items[0].GetDeletionTimestamp().IsZero()
	if instanceIsBeingDeleted {
		return nil
	}

	r.Log.Debugf("name: %s, ns: %s, gvk: %s, rscVer: %s, kymaRscVer: %s", object.GetName(), object.GetNamespace(), object.GetObjectKind().GroupVersionKind(), object.GetResourceVersion(), kymas.Items[0].ResourceVersion)

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: kymas.Items[0].Namespace,
				Name:      kymas.Items[0].Name,
			},
		},
	}
}

func (r *CompassManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Infoln("reconciliation triggered")

	//if call to Director will not succeed trigger call once again, and if failed again wait 2 minutes and repeat whole process
	return ctrl.Result{}, nil
}

func (r *CompassManagerReconciler) checkKubeconfigStrategy(obj runtime.Object) bool {

	kymaObj, ok := obj.(*kyma.Kyma)
	if !ok {
		r.Log.Errorf("%s", "cannot parse Kyma Custom Resource")
		return false
	}

	if kymaObj.Spec.Sync.Strategy == "" {
		r.Log.Errorf("%s", "Kubeconfig strategy not providied in Kyma Custom Resource")
		return false
	}

	return true
}

func (r *CompassManagerReconciler) checkUpdateKubeconfigStrategy(objNew, objOld runtime.Object) bool {

	kymaObjNew, okNew := objNew.(*kyma.Kyma)
	kymaObjOld, okOld := objOld.(*kyma.Kyma)

	if !okNew || !okOld {
		r.Log.Errorf("%s", "cannot parse Kyma Custom Resource")
		return false
	}

	if kymaObjNew.Spec.Sync.Strategy == "" || kymaObjOld.Spec.Sync.Strategy == "" {
		r.Log.Errorf("%s", "Kubeconfig strategy not providied in Kyma Custom Resource")
		return false
	}

	if strings.Compare(string(kymaObjNew.Spec.Sync.Strategy), string(kymaObjOld.Spec.Sync.Strategy)) == 0 {
		r.Log.Infof("%s", "Kubeconfig strategy has not changed, skipping reconcilation")
		return false
	}
	return true
}

// We can provide some logic to check if user updates the Kubeconfig e.g. field strategy -> check if secret with Kubeconfig is present in kcp-system?

// SetupWithManager sets up the controller with the Manager.
func (r *CompassManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelectorPredicate, err := predicate.LabelSelectorPredicate(
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "lifecycle-manager",
			},
		},
	)

	if err != nil {
		return err
	}

	fieldSelectorPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.checkKubeconfigStrategy(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.checkUpdateKubeconfigStrategy(e.ObjectNew, e.ObjectOld)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return r.checkKubeconfigStrategy(e.Object)
		},
		DeleteFunc: nil,
	}

	//From documentation
	/*
		// For defines the type of Object being *reconciled*, and configures the ControllerManagedBy to respond to create / delete /
		// update events by *reconciling the object*.
		// This is the equivalent of calling
		// Watches(&source.Kind{Type: apiType}, &handler.EnqueueRequestForObject{}).
	*/
	runner := ctrl.NewControllerManagedBy(mgr)

	watcher := func(u kyma.Kyma) {
		r.Log.Infoln("gvk", u.GroupVersionKind().String(), " adding watcher")
		runner = runner.Watches(
			&source.Kind{Type: &u},
			handler.EnqueueRequestsFromMapFunc(r.mapFunction),
			builder.WithPredicates(
				predicate.And(
					predicate.ResourceVersionChangedPredicate{},
					labelSelectorPredicate,
				),
			),
		)
	}

	if err := registerWatchDistinct(r.KymaObjs, watcher); err != nil {
		return err
	}

	return runner.WithEventFilter(fieldSelectorPredicate).Complete(r)
}
