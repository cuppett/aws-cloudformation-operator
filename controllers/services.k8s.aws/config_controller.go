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
	coreerrors "errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	servicesv1alpha1 "github.com/cuppett/aws-cloudformation-operator/apis/services.k8s.aws/v1alpha1"
	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	ccmv1 "github.com/openshift/cloud-credential-operator/pkg/apis/cloudcredential/v1"
	v12 "k8s.io/api/core/v1"
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

const (
	configName       string = "default"
	credReqName      string = "aws-cloudformation-controller-keys"
	credReqNamespace string = "openshift-cloud-credential-operator"
	credSecretName   string = "aws-cloud-credentials"
)

var (
	podNamespace      string = os.Getenv("POD_NAMESPACE")
	podServiceAccount string = os.Getenv("POD_SERVICE_ACCOUNT")
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client         client.Client
	log            logr.Logger
	scheme         *runtime.Scheme
	cloudFormation *cloudformation.Client
	cfLock         sync.Mutex
}

func InitializeConfigReconciler(client client.Client, log logr.Logger, scheme *runtime.Scheme) *ConfigReconciler {
	reconciler := &ConfigReconciler{
		client:         client,
		log:            log,
		scheme:         scheme,
		cloudFormation: nil,
	}
	return reconciler
}

type ConfigLoop struct {
	ctx    context.Context
	config *servicesv1alpha1.Config
	Log    logr.Logger
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

	loop := &ConfigLoop{ctx, &servicesv1alpha1.Config{},
		log.FromContext(ctx).WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)}

	// Fetch the Stack instance
	err := r.client.Get(loop.ctx, req.NamespacedName, loop.config)
	if err != nil {
		loop.Log.Error(err, "Failed to get Config")
		return ctrl.Result{}, err
	}

	r.cfLock.Lock()
	r.createCloudFormation(loop)
	r.cfLock.Unlock()

	r.createOrUpdateCredentialRequest(loop)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Currently only watching the one default config in the running pod podNamespace.
	returnVal := ctrl.NewControllerManagedBy(mgr).
		For(&servicesv1alpha1.Config{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				if podNamespace == e.Object.GetNamespace() && e.Object.GetName() == configName {
					return true
				}
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if podNamespace == e.ObjectOld.GetNamespace() && e.ObjectOld.GetName() == configName {
					return true
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				if podNamespace == e.Object.GetNamespace() && e.Object.GetName() == configName {
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

func (r *ConfigReconciler) GetTags(ctx context.Context) map[string]string {

	defaultConfig := r.getDefaultConfig(ctx)
	if defaultConfig != nil {
		return defaultConfig.Spec.Tags
	}
	return map[string]string{}

}

func (r *ConfigReconciler) GetCloudFormation() *cloudformation.Client {
	if r.cloudFormation == nil {
		r.cfLock.Lock()
		if r.cloudFormation == nil {
			ctx := context.TODO()
			loop := &ConfigLoop{ctx, r.getDefaultConfig(ctx),
				log.FromContext(ctx)}
			r.createCloudFormation(loop)
		}
		r.cfLock.Unlock()
	}
	return r.cloudFormation
}

func (r *ConfigReconciler) createCloudFormation(loop *ConfigLoop) {
	cfg := r.loadConfig(loop)
	r.cloudFormation = cloudformation.NewFromConfig(*cfg)
}

func (r *ConfigReconciler) loadConfig(loop *ConfigLoop) *aws.Config {

	cfg, err := config.LoadDefaultConfig(loop.ctx)
	if err != nil {
		r.log.Error(err, "error getting AWS config")
		return nil
	}

	if cfg.Region == "" {
		cfg.Region = r.getClusterRegion(loop)
	}

	r.log.Info("Region resolved", "region", cfg.Region)

	// Looking for credentials
	credentials, err := cfg.Credentials.Retrieve(loop.ctx)
	// If the default ways didn't give it to us and we're in OpenShift Mint Mode
	if (err != nil || !credentials.HasKeys()) && r.isMintMode(loop.ctx) {
		secret := &v12.Secret{}
		namespacedName := types.NamespacedName{
			Namespace: podNamespace,
			Name:      credSecretName,
		}
		err = r.client.Get(loop.ctx, namespacedName, secret)
		if err != nil {
			r.log.Info("Failed to get Secret", "error", err)
			r.createOrUpdateCredentialRequest(loop)
		} else {
			cfg.Credentials = &SecretProvider{
				*secret,
			}
		}
	}

	// Log the identity being used
	stsClient := sts.NewFromConfig(cfg)
	output, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil || output == nil {
		r.log.Info("No AWS identity available in config.", "error", err)
	} else {
		r.log.Info("AWS identity found", "arn", *output.Arn)
	}

	return &cfg
}

//+kubebuilder:rbac:groups=cloudcredential.openshift.io,resources=credentialsrequests,verbs=get;list;watch

func (r *ConfigReconciler) createOrUpdateCredentialRequest(loop *ConfigLoop) {
	credentialRequest := &ccmv1.CredentialsRequest{}
	namespacedName := types.NamespacedName{
		Name:      credReqName,
		Namespace: credReqNamespace,
	}
	err := r.client.Get(loop.ctx, namespacedName, credentialRequest)
	if err != nil {
		if errors.IsNotFound(err) {
			r.log.Info("CredentialsRequest does not exist")
			credentialRequest.Name = namespacedName.Name
			credentialRequest.Namespace = namespacedName.Namespace
			r.createCredentialRequest(loop, credentialRequest)
		} else {
			r.log.Error(err, "Failed to get CredentialRequest")
		}
	} else {
		r.updateCredentialRequest(loop, credentialRequest)
		r.log.Info("CredentialsRequest exists", "status", credentialRequest.Status)
	}
}

//+kubebuilder:rbac:groups=cloudcredential.openshift.io,resources=credentialsrequests,verbs=create

func (r *ConfigReconciler) createCredentialRequest(loop *ConfigLoop, credentialRequest *ccmv1.CredentialsRequest) {

	credentialRequest.Spec.SecretRef.Name = credSecretName
	credentialRequest.Spec.SecretRef.Namespace = podNamespace
	credentialRequest.Spec.ServiceAccountNames = []string{podServiceAccount}

	err := r.updateCredentialsRequestPolicyStatements(loop, credentialRequest)

	if err == nil {
		err = r.client.Create(loop.ctx, credentialRequest)
	}
	if err != nil {
		r.log.Error(err, "Failure creating CredentialsRequest object")
	} else {
		r.log.Info("Created CredentialsRequest")
	}
}

//+kubebuilder:rbac:groups=cloudcredential.openshift.io,resources=credentialsrequests,verbs=update;patch

func (r *ConfigReconciler) updateCredentialRequest(loop *ConfigLoop, credentialRequest *ccmv1.CredentialsRequest) {

	err := r.updateCredentialsRequestPolicyStatements(loop, credentialRequest)

	if err == nil {
		r.log.Info("updating with", "credentialrequest", credentialRequest)
		err = r.client.Update(loop.ctx, credentialRequest)
	}
	if err != nil {
		r.log.Error(err, "Failure updating CredentialsRequest object")
	} else {
		r.log.Info("Updated CredentialsRequest")
	}
}

func (r *ConfigReconciler) updateCredentialsRequestPolicyStatements(loop *ConfigLoop, credentialRequest *ccmv1.CredentialsRequest) error {

	credentialRequestProviderSpec := ccmv1.AWSProviderSpec{}
	credentialRequestProviderSpec.StatementEntries = []ccmv1.StatementEntry{
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"iam:PassRole",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"cloudformation:CreateStack",
				"cloudformation:DescribeStackInstance",
				"cloudformation:DescribeStackResource",
				"cloudformation:DescribeStacks",
				"cloudformation:ListStackResources",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"cloudformation:DeleteStack",
				"cloudformation:UpdateStack",
			},
			PolicyCondition: ccmv1.IAMPolicyCondition{
				"StringEquals": ccmv1.IAMPolicyConditionKeyValue{
					"aws:ResourceTag/kubernetes.io/controlled-by": "cloudformation.services.k8s.aws.cuppett.dev/controller",
				},
			},
		},
	}

	codec, err := ccmv1.NewCodec()
	if err != nil {
		r.log.Error(err, "Failure creating codec")
	}

	// Pick up any additional permissions required to be minted in via OpenShift
	if loop.config != nil {
		if loop.config.Spec.AdditionalPermissions != nil {
			awsSpec := ccmv1.AWSProviderSpec{}
			err = codec.DecodeProviderSpec(loop.config.Spec.AdditionalPermissions, &awsSpec)
			if err == nil {
				for _, entry := range awsSpec.StatementEntries {
					r.log.Info("Appending in something", "entry", entry)
					credentialRequestProviderSpec.StatementEntries = append(credentialRequestProviderSpec.StatementEntries, entry)
				}
				if len(awsSpec.StatementEntries) == 0 {
					r.log.Info("There are no entries", "awsSpec", awsSpec)
				}
			} else {
				r.log.Error(err, "Failed to decode.")
			}
		} else {
			r.log.Error(err, "No additional permissions.")
		}
	} else {
		err = coreerrors.New("Config is nil")
		r.log.Error(err, "Config could not be retrieved")
	}

	r.log.Info("Our current providerspec", "spec", credentialRequestProviderSpec)

	var raw *runtime.RawExtension
	if err == nil {
		raw, err = codec.EncodeProviderSpec(credentialRequestProviderSpec.DeepCopyObject())
	}
	if err == nil && raw != nil {
		credentialRequest.Spec.ProviderSpec = raw
	} else if raw == nil {
		err = coreerrors.New("Failed to marshal the raw object into a ProviderSpec")
	}

	return err
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
		Namespace: podNamespace,
		Name:      configName,
	})
	return defaultConfig
}

func (r *ConfigReconciler) getClusterRegion(loop *ConfigLoop) string {
	// Look for direct configuration via our Config CRD
	if loop.config != nil && loop.config.Spec.Region != "" {
		return loop.config.Spec.Region
	}

	// Lastly, if we're on OpenShift, check what the region is for the infra.
	return r.getInfraRegion(loop.ctx)
}

//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get;list;watch

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

//+kubebuilder:rbac:groups=operator.openshift.io,resources=cloudcredentials,verbs=get;list;watch

func (r *ConfigReconciler) isMintMode(ctx context.Context) bool {
	var err error
	var gvkCc v1.GroupVersionKind
	var gvkCr v1.GroupVersionKind

	gvkCc.Kind = "CloudCredential"
	gvkCc.Group = "operator.openshift.io"
	gvkCc.Version = "v1"
	gvkCr.Kind = "CredentialsRequest"
	gvkCr.Group = "cloudcredential.openshift.io"
	gvkCr.Version = "v1"

	// Ensure both CloudCredential and CredentialsRequest are on the system, then check the mint mode on CloudCredential
	if r.crdExists(ctx, "cloudcredentials.operator.openshift.io", gvkCc) &&
		r.crdExists(ctx, "credentialsrequests.cloudcredential.openshift.io", gvkCr) {
		cc := &operatorv1.CloudCredential{}
		err = r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, cc)
		if err != nil {
			r.log.Error(err, "Failed to get defined credential mode from OpenShift")
			return false
		}
		if cc.Spec.CredentialsMode == operatorv1.CloudCredentialsModeDefault || cc.Spec.CredentialsMode == operatorv1.CloudCredentialsModeMint {
			return true
		} else {
			r.log.Info("Deployment mode not mint", "mode", cc.Spec.CredentialsMode)
		}
	}
	return false
}

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

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
