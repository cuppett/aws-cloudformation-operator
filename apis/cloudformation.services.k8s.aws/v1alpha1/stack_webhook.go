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

package v1alpha1

import (
	coreerrors "errors"
	"k8s.io/apimachinery/pkg/runtime"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var stacklog = logf.Log.WithName("stack-resource")

func (r *Stack) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var (
	ErrBothTemplateAndUrl = coreerrors.New("Template and TemplateUrl cannot both be provided")
	ErrNeedTemplateOrUrl  = coreerrors.New("Template or TemplateUrl must be provided")
	ErrMissingRole        = coreerrors.New("Role cannot be omitted on update")
	ErrRoleArnTooShort    = coreerrors.New("Role ARN length must be at least 20 characters.")
	ErrCannotChangeOnFail = coreerrors.New("You cannot change/update onFailure after create.")
	ErrTooManyARNs        = coreerrors.New("You cannot specify more than 5 NotificationARNs.")
	ErrCannotRenameStacks = coreerrors.New("You cannot change the name of a stack after creation.")
	ErrStackNameTooLong   = coreerrors.New("Stack names limited to 64 characters.")
	ErrBadCapability      = coreerrors.New("Invalid capability specified.")
	allowedCapabilities   = []string{"CAPABILITY_IAM", "CAPABILITY_NAMED_IAM", "CAPABILITY_AUTO_EXPAND"}
	ErrStackNameFormat    = coreerrors.New("Stack name can include letters (A-Z and a-z), numbers (0-9), and dashes (-). Must start with a letter.")
	nameRegex, _          = regexp.Compile("^[a-zA-Z][a-zA-Z0-9\\-]*$")
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-cloudformation-services-k8s-aws-cuppett-dev-v1alpha1-stack,mutating=false,failurePolicy=fail,sideEffects=None,groups=cloudformation.services.k8s.aws.cuppett.dev,resources=stacks,verbs=create;update,versions=v1alpha1,name=vstack.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Stack{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Stack) ValidateCreate() (admission.Warnings, error) {
	stacklog.Info("validate create", "name", r.Name)

	// Checking to ensure both the template and templateUrl aren't specified.
	if r.Spec.Template != "" && r.Spec.TemplateUrl != "" {
		return nil, ErrBothTemplateAndUrl
	}

	// Ensuring either the template or templateUrl are specified.
	if r.Spec.Template == "" && r.Spec.TemplateUrl == "" {
		return nil, ErrNeedTemplateOrUrl
	}

	// Ensuring the Role ARN is long enough
	if r.Spec.RoleARN != "" && len(r.Spec.RoleARN) < 20 {
		return nil, ErrRoleArnTooShort
	}

	// Ensuring no more than 5 NotificationARNs are submitted
	if len(r.Spec.NotificationArns) > 5 {
		return nil, ErrTooManyARNs
	}

	if len(r.Spec.StackName) > 64 {
		return nil, ErrStackNameTooLong
	}

	if r.Spec.StackName != "" && !nameRegex.Match([]byte(r.Spec.StackName)) {
		return nil, ErrStackNameFormat
	}

	// Ensuring the capabilities input are within the known/allowed set
	for _, x := range r.Spec.Capabilities {
		goodCapability := false
		for _, y := range allowedCapabilities {
			if x == y {
				goodCapability = true
				break
			}
		}
		if !goodCapability {
			return nil, ErrBadCapability
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Stack) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	stacklog.Info("validate update", "name", r.Name)

	// Converting to Stack type
	oldStack := old.(*Stack)

	if r.Spec.RoleARN == "" && oldStack.Status.RoleARN != "" {
		return nil, ErrMissingRole
	}

	if r.Spec.OnFailure != oldStack.Spec.OnFailure {
		return nil, ErrCannotChangeOnFail
	}

	if r.Spec.StackName != oldStack.Spec.StackName {
		return nil, ErrCannotRenameStacks
	}

	return r.ValidateCreate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Stack) ValidateDelete() (admission.Warnings, error) {
	stacklog.Info("validate delete", "name", r.Name)

	// Nothing presently problematic about delete.

	return nil, nil
}
