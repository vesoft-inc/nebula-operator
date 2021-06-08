/*
Copyright 2021 Vesoft Inc.

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

package validating

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

// NebulaClusterCreateUpdateHandler handles StatefulSet
type NebulaClusterCreateUpdateHandler struct {
	// To use the client, you need to do the following:
	// - uncomment it
	// - import sigs.k8s.io/controller-runtime/pkg/client
	// - uncomment the InjectClient method at the bottom of this file.
	Client client.Client

	// Decoder decodes objects
	Decoder *admission.Decoder
}

var _ admission.Handler = &NebulaClusterCreateUpdateHandler{}

// Handle handles admission requests.
func (h *NebulaClusterCreateUpdateHandler) Handle(_ context.Context, req admission.Request) admission.Response {
	klog.Infof("validating %s [%s/%s] on %s", req.Resource, req.Namespace, req.Name, req.Operation)

	obj := &v1alpha1.NebulaCluster{}

	err := h.Decoder.Decode(req, obj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	operation := req.AdmissionRequest.Operation

	// If not Create or Update, return earlier.
	if !(operation == admissionv1.Create || operation == admissionv1.Update) {
		return admission.ValidationResponse(true, "")
	}

	if operation == admissionv1.Create {
		if allErrs := validateNebulaClusterCreate(obj); len(allErrs) > 0 {
			return admission.Errored(http.StatusUnprocessableEntity, allErrs.ToAggregate())
		}
	} else if operation == admissionv1.Update {
		oldObj := &v1alpha1.NebulaCluster{}

		if err := h.Decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if allErrs := validateNebulaClusterUpdate(obj, oldObj); len(allErrs) > 0 {
			return admission.Errored(http.StatusUnprocessableEntity, allErrs.ToAggregate())
		}
	}

	return admission.ValidationResponse(true, "")
}

var _ inject.Client = &NebulaClusterCreateUpdateHandler{}

// InjectClient injects the client into the NebulaClusterCreateUpdateHandler
func (h *NebulaClusterCreateUpdateHandler) InjectClient(c client.Client) error {
	h.Client = c
	return nil
}

var _ admission.DecoderInjector = &NebulaClusterCreateUpdateHandler{}

// InjectDecoder injects the decoder into the NebulaClusterCreateUpdateHandler
func (h *NebulaClusterCreateUpdateHandler) InjectDecoder(d *admission.Decoder) error {
	h.Decoder = d
	return nil
}
