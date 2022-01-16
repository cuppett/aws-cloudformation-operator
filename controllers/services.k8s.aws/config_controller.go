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
	servicesv1alpha1 "github.com/cuppett/aws-cloudformation-controller/apis/services.k8s.aws/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
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
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Currently only watching the one default config in the running pod namespace.
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicesv1alpha1.Config{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				if os.Getenv("POD_NAMESPACE") == e.Object.GetNamespace() && e.Object.GetName() == "default" {
					return true
				}
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if os.Getenv("POD_NAMESPACE") == e.ObjectOld.GetNamespace() && e.ObjectOld.GetName() == "default" {
					return true
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				if os.Getenv("POD_NAMESPACE") == e.Object.GetNamespace() && e.Object.GetName() == "default" {
					return true
				}
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}).
		Complete(r)
}

func (r *ConfigReconciler) retrieveConfig(ctx context.Context, name types.NamespacedName) (*servicesv1alpha1.Config, error) {
	// Fetch the Config instance
	toReturn := &servicesv1alpha1.Config{}
	err := r.Client.Get(ctx, name, toReturn)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, may never have been created.
			r.Log.Info("Default config resource not found. Ignoring since object must not exist.")
			return nil, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get default config.")
		return nil, err
	}
	return toReturn, nil
}
