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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "ingresstrait/api/v1alpha1"
)

// Reconcile error strings.
const (
	errLocateWorkload    = "cannot find workload"
	errLocateResources   = "cannot find resources"
	errLocateStatefulSet = "cannot find statefulset"
)

// IngressTraitReconciler reconciles a IngressTrait object
type IngressTraitReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.oam.dev,resources=ingresstraits,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.oam.dev,resources=ingresstraits/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.oam.dev,resources=statefulsetworkloads,verbs=get;list;
// +kubebuilder:rbac:groups=core.oam.dev,resources=statefulsetworkloads/status,verbs=get;
// +kubebuilder:rbac:groups=apps,resources=statefulset,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *IngressTraitReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("ingresstrait", req.NamespacedName)
	log.Info("Reconcile Ingress Trait")

	var ingress corev1alpha1.IngressTrait
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Get the ingress trait", "WorkloadReference", ingress.Spec.WorkloadReference)

	//2020.5.19 21:00

	return ctrl.Result{}, nil
}

func (r *IngressTraitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.IngressTrait{}).
		Complete(r)
}
