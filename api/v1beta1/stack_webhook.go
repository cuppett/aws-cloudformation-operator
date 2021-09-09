/*
MIT License

Copyright (c) 2021 Stephen Cuppett

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

package v1beta1

import (
	coreerrors "errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-cloudformation-cuppett-com-v1beta1-stack,mutating=false,failurePolicy=fail,sideEffects=None,groups=cloudformation.cuppett.com,resources=stacks,verbs=create;update,versions=v1beta1,name=vstack.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Stack{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Stack) ValidateCreate() error {
	stacklog.Info("validate create", "name", r.Name)

	// Checking to ensure both the template and templateUrl aren't specified.
	if r.Spec.Template != "" && r.Spec.TemplateUrl != "" {
		return ErrBothTemplateAndUrl
	}

	// Ensuring either the template or templateUrl are specified.
	if r.Spec.Template == "" && r.Spec.TemplateUrl == "" {
		return ErrNeedTemplateOrUrl
	}

	// Ensuring the Role ARN is long enough
	if r.Spec.RoleARN != "" && len(r.Spec.RoleARN) < 20 {
		return ErrRoleArnTooShort
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Stack) ValidateUpdate(old runtime.Object) error {
	stacklog.Info("validate update", "name", r.Name)

	// Converting to Stack type
	oldStack := old.(*Stack)

	if r.Spec.RoleARN == "" && oldStack.Status.RoleARN != "" {
		return ErrMissingRole
	}

	if r.Spec.OnFailure != oldStack.Spec.OnFailure {
		return ErrCannotChangeOnFail
	}

	return r.ValidateCreate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Stack) ValidateDelete() error {
	stacklog.Info("validate delete", "name", r.Name)

	// Nothing presently problematic about delete.

	return nil
}
