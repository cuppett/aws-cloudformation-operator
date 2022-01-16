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

package cloudformation_services_k8s_aws

import (
	"context"
	"github.com/cuppett/aws-cloudformation-controller/apis/cloudformation.services.k8s.aws/v1alpha1"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type MapWriter struct {
	client.Client
	Log logr.Logger
	ChannelHub
	*runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Worker
func (w *MapWriter) Worker() {

	for {
		toBeMapped := <-w.ChannelHub.MappingChannel
		w.Log.Info("Synchronizing map", "Namespace", toBeMapped.Namespace, "Stack ID", toBeMapped.Status.StackID)

		// Getting the map
		m := &v1.ConfigMap{}
		created := false
		namespacedName := types.NamespacedName{Namespace: toBeMapped.Namespace, Name: toBeMapped.Name + "-cm"}
		err := w.Client.Get(context.TODO(), namespacedName, m)
		if errors.IsNotFound(err) {
			w.Log.Info("Map resource not found. To be created.", "Namespace", toBeMapped.Namespace, "Stack ID", toBeMapped.Status.StackID)
			*m = w.createMap(toBeMapped)
			created = true
		}

		// Writing map outputs
		m.Data = toBeMapped.Status.Outputs

		// Setting the owner reference
		err = controllerutil.SetControllerReference(toBeMapped, m, w.Scheme)
		if err != nil {
			w.Log.Error(err, "Unable to set controller owner.", "Namespace", toBeMapped.Namespace, "Name", m.Name, "Stack ID", toBeMapped.Status.StackID)
		} else {
			if created {
				err = w.Client.Create(context.TODO(), m)
			} else {
				err = w.Client.Update(context.TODO(), m)
			}
		}

		if err != nil {
			w.Log.Error(err, "Failed to create or update map.", "Namespace", toBeMapped.Namespace, "Name", m.Name, "Stack ID", toBeMapped.Status.StackID)
		} else {
			w.Log.Info("Map written", "Namespace", toBeMapped.Namespace, "Name", m.Name, "Stack ID", toBeMapped.Status.StackID)
		}
	}
}

func (w *MapWriter) createMap(stack *v1alpha1.Stack) v1.ConfigMap {
	return v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stack.Name + "-cm",
			Namespace: stack.Namespace,
		},
	}
}
