/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	cpv1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/oam-kubernetes-runtime/pkg/oam"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"github.com/crossplane/oam-controllers/pkg/oam/util"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha2 "ingresstrait/api/v1alpha2"
)

const (
	oamReconcileWait = 30 * time.Second
)

// Reconcile error strings.
const (
	errLocateWorkload    = "cannot find workload"
	errLocateResources   = "cannot find resources"
	errUpdateStatus      = "cannot apply status"
	errLocateStatefulSet = "cannot find statefulset"
	errApplyIngress      = "cannot apply the ingress"
	errGCIngress         = "cannot clean up stale ingress"
)

// IngressTraitReconciler reconciles a IngressTrait object
type IngressTraitReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.oam.dev,resources=ingresstraits,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.oam.dev,resources=ingresstraits/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.oam.dev,resources=statefulsetworkloads,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.oam.dev,resources=statefulsetworkloads/status,verbs=get;
// +kubebuilder:rbac:groups=core.oam.dev,resources=workloaddefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *IngressTraitReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("ingresstrait", req.NamespacedName)
	log.Info("Reconcile Ingress Trait")

	var ingressTr corev1alpha2.IngressTrait
	if err := r.Get(ctx, req.NamespacedName, &ingressTr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Get the ingress trait", "WorkloadReference", ingressTr.Spec.WorkloadReference)

	// Fetch the workload this trait is referring to
	workload, result, err := r.fetchWorkload(ctx, &ingressTr)
	if err != nil {
		return result, err
	}

	// Fetch the child resources list from the corresponding workload
	resources, err := util.FetchWorkloadDefinition(ctx, r, workload)
	if err != nil {
		r.Log.Error(err, "Cannot find the workload child resources", "workload", workload.UnstructuredContent())
		ingressTr.Status.SetConditions(cpv1alpha1.ReconcileError(fmt.Errorf(errLocateResources)))
		return ctrl.Result{RequeueAfter: oamReconcileWait}, errors.Wrap(r.Status().Update(ctx, &ingressTr),
			errUpdateStatus)
	}

	// Create a ingress for the child resources we know
	ingress, err := r.createIngress(ctx, ingressTr, resources)
	if err != nil {
		return result, err
	}

	// server side apply the ingress, only the fields we set are touched
	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner(ingressTr.Name)}
	if err := r.Patch(ctx, ingress, client.Apply, applyOpts...); err != nil {
		ingressTr.Status.SetConditions(cpv1alpha1.ReconcileError(errors.Wrap(err, errApplyIngress)))
		r.Log.Error(err, "Failed to apply a ingress")
		return reconcile.Result{RequeueAfter: oamReconcileWait}, errors.Wrap(r.Status().Update(ctx, &ingressTr),
			errUpdateStatus)
	}
	r.Log.Info("Successfully applied a ingress", "UID", ingress.UID)

	// garbage collect the ingress that we created but not needed
	if err := r.cleanupResources(ctx, &ingressTr, &ingress.UID); err != nil {
		ingressTr.Status.SetConditions(cpv1alpha1.ReconcileError(errors.Wrap(err, errGCIngress)))
		r.Log.Error(err, "Failed to clean up resources")
		return reconcile.Result{RequeueAfter: oamReconcileWait}, errors.Wrap(r.Status().Update(ctx, &ingressTr),
			errUpdateStatus)
	}
	ingressTr.Status.Resources = nil
	// record the new ingress
	ingressTr.Status.Resources = append(ingressTr.Status.Resources, cpv1alpha1.TypedReference{
		APIVersion: ingress.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       ingress.GetObjectKind().GroupVersionKind().Kind,
		Name:       ingress.GetName(),
		UID:        ingress.GetUID(),
	})

	ingressTr.Status.SetConditions(cpv1alpha1.ReconcileSuccess())

	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, &ingressTr), errUpdateStatus)
}

func (r *IngressTraitReconciler) createIngress(ctx context.Context, ingressTr corev1alpha2.IngressTrait,
	resources []*unstructured.Unstructured) (*v1beta1.Ingress, error) {
	// Change unstructured to object
	for _, res := range resources {
		if res.GetKind() == KindStatefulSet && res.GetAPIVersion() == appsv1.SchemeGroupVersion.String() {
			r.Log.Info("Get the statefulset the trait is going to create a ingress for it",
				"statefulset name", res.GetName(), "UID", res.GetUID())
			// convert the unstructured to statefulset and create a ingress
			var ss appsv1.StatefulSet
			bts, _ := json.Marshal(res)
			if err := json.Unmarshal(bts, &ss); err != nil {
				r.Log.Error(err, "Failed to convert an unstructured obj to a statefulset")
				continue
			}
			// Create a ingress for the workload which this trait is referring to
			ingress, err := r.renderIngress(ctx, &ingressTr, &ss)
			if err != nil {
				r.Log.Error(err, "Failed to render a ingress")
				return nil, errors.Wrap(r.Status().Update(ctx, &ingressTr),
					errUpdateStatus)
			}
			return ingress, nil
		}
	}
	r.Log.Info("Cannot locate any statefulset", "total resources", len(resources))
	ingressTr.Status.SetConditions(cpv1alpha1.ReconcileError(fmt.Errorf(errLocateResources)))
	return nil, errors.Wrap(r.Status().Update(ctx, &ingressTr),
		errUpdateStatus)
}

func (r *IngressTraitReconciler) fetchWorkload(ctx context.Context,
	oamTrait oam.Trait) (*unstructured.Unstructured, ctrl.Result, error) {
	var workload unstructured.Unstructured
	workload.SetAPIVersion(oamTrait.GetWorkloadReference().APIVersion)
	workload.SetKind(oamTrait.GetWorkloadReference().Kind)
	wn := client.ObjectKey{Name: oamTrait.GetWorkloadReference().Name, Namespace: oamTrait.GetNamespace()}
	if err := r.Get(ctx, wn, &workload); err != nil {
		oamTrait.SetConditions(cpv1alpha1.ReconcileError(errors.Wrap(err, errLocateWorkload)))
		r.Log.Error(err, "Workload not find", "kind", oamTrait.GetWorkloadReference().Kind,
			"worklaod name", oamTrait.GetWorkloadReference().Name)
		return nil, ctrl.Result{RequeueAfter: oamReconcileWait}, errors.Wrap(r.Status().Update(ctx, oamTrait),
			errUpdateStatus)
	}
	r.Log.Info("Get the workload the trait is pointing to", "workload name", oamTrait.GetWorkloadReference().Name,
		"UID", oamTrait.GetWorkloadReference().UID)
	return &workload, ctrl.Result{}, nil
}

func (r *IngressTraitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha2.IngressTrait{}).
		Complete(r)
}
