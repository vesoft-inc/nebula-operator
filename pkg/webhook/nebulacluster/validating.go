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

package nebulacluster

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

// ValidatingAdmission handles StatefulSet
type ValidatingAdmission struct {
	// Decoder decodes objects
	Decoder *admission.Decoder
}

var _ admission.Handler = &ValidatingAdmission{}

// Handle handles admission requests.
func (h *ValidatingAdmission) Handle(_ context.Context, req admission.Request) (resp admission.Response) {
	klog.Infof("start validating resource %v [%s/%s] operation %s", req.Resource, req.Namespace, req.Name, req.Operation)

	defer func() {
		klog.Infof("end validating, allowed %v, reason %v, message %s", resp.Allowed,
			resp.Result.Reason, resp.Result.Message)
	}()

	obj := &v1alpha1.NebulaCluster{}

	if req.Operation == admissionv1.Delete {
		if err := h.Decoder.DecodeRaw(req.OldObject, obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := h.Decoder.Decode(req, obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	operation := req.AdmissionRequest.Operation

	if operation == admissionv1.Connect {
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
	} else if operation == admissionv1.Delete {
		if allErrs := validateNebulaClusterDelete(obj); len(allErrs) > 0 {
			return admission.Errored(http.StatusUnprocessableEntity, allErrs.ToAggregate())
		}
	}

	return admission.ValidationResponse(true, "")
}
