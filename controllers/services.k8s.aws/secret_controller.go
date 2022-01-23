/*
MIT License

Copyright (c) 2022 Stephen Cuppett

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package servicesk8saws

import (
	"context"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Secret reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Log logr.Logger
	*runtime.Scheme
	*ConfigReconciler
}

//+kubebuilder:rbac:groups=services.k8s.aws.cuppett.dev,resources=configs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=services.k8s.aws.cuppett.dev,resources=configs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=services.k8s.aws.cuppett.dev,resources=configs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Config object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile

// +kubebuilder:rbac:groups=*,resources=secrets,verbs=list;watch

func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	r.ConfigReconciler.Reconcile(ctx, req)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Currently only watching the one default secret in the running pod namespace.
	returnVal := ctrl.NewControllerManagedBy(mgr).
		For(&v1.Secret{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				if podNamespace == e.Object.GetNamespace() && e.Object.GetName() == credSecretName {
					return true
				}
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if podNamespace == e.ObjectOld.GetNamespace() && e.ObjectOld.GetName() == credSecretName {
					return true
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				if podNamespace == e.Object.GetNamespace() && e.Object.GetName() == credSecretName {
					return true
				}
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}).
		Complete(r)
	return returnVal
}
