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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	servicesv1alpha1 "github.com/cuppett/aws-cloudformation-controller/apis/services.k8s.aws/v1alpha1"
	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client         client.Client
	log            logr.Logger
	scheme         *runtime.Scheme
	assumeRole     string
	cloudFormation *cloudformation.Client
	cfLock         sync.Mutex
}

func InitializeConfigReconciler(client client.Client, log logr.Logger, scheme *runtime.Scheme, role string) *ConfigReconciler {
	reconciler := &ConfigReconciler{
		client:         client,
		log:            log,
		scheme:         scheme,
		assumeRole:     role,
		cloudFormation: nil,
	}
	return reconciler
}

//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get;list;watch
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
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
	r.cfLock.Lock()
	r.createCloudFormation()
	r.cfLock.Unlock()
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Currently only watching the one default config in the running pod namespace.
	returnVal := ctrl.NewControllerManagedBy(mgr).
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
	return returnVal
}

func (r *ConfigReconciler) GetCloudFormation() *cloudformation.Client {
	if r.cloudFormation == nil {
		r.cfLock.Lock()
		if r.cloudFormation == nil {
			r.createCloudFormation()
		}
		r.cfLock.Unlock()
	}
	return r.cloudFormation
}

func (r *ConfigReconciler) createCloudFormation() {
	cfg := r.loadConfig(context.TODO())
	r.cloudFormation = cloudformation.NewFromConfig(*cfg)
}

func (r *ConfigReconciler) loadConfig(ctx context.Context) *aws.Config {

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		r.log.Error(err, "error getting AWS config")
		return nil
	}

	if cfg.Region == "" {
		cfg.Region = r.getClusterRegion(ctx)
	}

	r.log.Info("Region resolved", "region", cfg.Region)

	stsClient := sts.NewFromConfig(cfg)

	if r.assumeRole != "" {
		r.log.Info("Injecting assume role provider for " + r.assumeRole)
		cfg.Credentials = stscreds.NewAssumeRoleProvider(stsClient, r.assumeRole)
		stsClient = sts.NewFromConfig(cfg)
	}

	// Log out the identity being used
	output, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil || output == nil {
		r.log.Error(err, "No AWS identity discovered in config.")
	} else {
		r.log.Info("AWS identity found", "arn", *output.Arn)
	}

	return &cfg
}

func (r *ConfigReconciler) retrieveConfig(ctx context.Context, name types.NamespacedName) (*servicesv1alpha1.Config, error) {
	// Fetch the Config instance
	toReturn := &servicesv1alpha1.Config{}
	err := r.client.Get(ctx, name, toReturn)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, may never have been created.
			r.log.Info("Default config resource not found. Ignoring since object must not exist.")
			return nil, nil
		}
		// Error reading the object - requeue the request.
		r.log.Error(err, "Failed to get default config.")
		return nil, err
	}

	return toReturn, nil
}

func (r *ConfigReconciler) getDefaultConfig(ctx context.Context) *servicesv1alpha1.Config {
	defaultConfig, _ := r.retrieveConfig(ctx, types.NamespacedName{
		Namespace: os.Getenv("POD_NAMESPACE"),
		Name:      "default",
	})
	return defaultConfig
}

func (r *ConfigReconciler) getClusterRegion(ctx context.Context) string {
	// Look for direct configuration via our Config CRD
	defaultConfig := r.getDefaultConfig(ctx)
	if defaultConfig != nil && defaultConfig.Spec.Region != "" {
		return defaultConfig.Spec.Region
	}

	// Lastly, if we're on OpenShift, check what the region is for the infra.
	return r.getInfraRegion(ctx)
}

func (r *ConfigReconciler) getInfraRegion(ctx context.Context) string {
	var err error
	var gvk v1.GroupVersionKind

	gvk.Kind = "Infrastructure"
	gvk.Group = "config.openshift.io"
	gvk.Version = "v1"

	if r.crdExists(ctx, "infrastructures.config.openshift.io", gvk) {
		infra := &configv1.Infrastructure{}
		err = r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, infra)
		if err != nil {
			r.log.Error(err, "Failed to get defined infrastructure from OpenShift")
			return ""
		}
		if infra.Status.PlatformStatus.Type == configv1.AWSPlatformType {
			return infra.Status.PlatformStatus.AWS.Region
		} else {
			r.log.Info("Deployed infrastructure not AWS", "type", infra.Status.PlatformStatus.Type)
		}
	}
	return ""
}

func (r *ConfigReconciler) crdExists(ctx context.Context, name string, gvk v1.GroupVersionKind) bool {
	crd := &apiextensions.CustomResourceDefinition{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name: name,
	}, crd)
	if err != nil {
		r.log.Info("No type on the system.", "name", name, "gvk", gvk)
		return false
	}

	if crd.Spec.Group != gvk.Group {
		r.log.Info("Group mismatch.", "gvk", gvk, "crd", crd)
		return false
	}
	if crd.Spec.Names.Kind != gvk.Kind {
		r.log.Info("Kind mismatch.", "gvk", gvk, "crd", crd)
		return false
	}

	found := false
	for _, version := range crd.Spec.Versions {
		if version.Name == gvk.Version {
			found = true
			break
		}
	}
	if !found {
		if crd.Spec.Names.Kind != gvk.Kind {
			r.log.Info("Versions mismatch.", "gvk", gvk, "crd", crd)
			return false
		}
	}
	return found
}
