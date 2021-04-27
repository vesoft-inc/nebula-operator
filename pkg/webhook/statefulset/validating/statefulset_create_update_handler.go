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
	"fmt"
	"net/http"

	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// StatefulSetCreateUpdateHandler handles StatefulSet
type StatefulSetCreateUpdateHandler struct {
	// To use the client, you need to do the following:
	// - uncomment it
	// - import sigs.k8s.io/controller-runtime/pkg/client
	// - uncomment the InjectClient method at the bottom of this file.
	Client client.Client

	// Decoder decodes objects
	Decoder *admission.Decoder
}

var _ admission.Handler = &StatefulSetCreateUpdateHandler{}

// Handle handles admission requests.
func (h *StatefulSetCreateUpdateHandler) Handle(ctx context.Context, req admission.Request) (resp admission.Response) {
	klog.Infof("start validating %s [%s/%s] on %s",
		req.Resource, req.Namespace, req.Name, req.Operation)
	defer func() {
		klog.Infof("end validating %s [%s/%s] on %s, {Allowed: %t, Reason: %s, Message: %s}",
			req.Resource, req.Namespace, req.Name, req.Operation, resp.Allowed, resp.Result.Reason, resp.Result.Message)
	}()

	var obj client.Object = &appsv1.StatefulSet{}
	if req.Resource.Group == kruisev1alpha1.GroupVersion.Group {
		obj = &kruisev1alpha1.StatefulSet{}
	}

	bNotScale := true
	if req.SubResource != "" {
		if req.SubResource != "scale" {
			return admission.ValidationResponse(true, "")
		}
		bNotScale = false
	} else {
		err := h.Decoder.Decode(req, obj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	operation := req.AdmissionRequest.Operation

	// If not Create or Update, return earlier.
	if !(operation == admissionv1.Create || operation == admissionv1.Update) {
		return admission.ValidationResponse(true, "")
	}

	if operation == admissionv1.Create {
		sts, err := convToSts(obj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if allErrs := validateStatefulSetCreate(sts); len(allErrs) > 0 {
			return admission.Errored(http.StatusUnprocessableEntity, allErrs.ToAggregate())
		}
	} else if operation == admissionv1.Update {
		oldObj := obj.DeepCopyObject().(client.Object)
		if bNotScale {
			if err := h.Decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldObj); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		} else {
			scale := autoscalingv1.Scale{}
			err := h.Decoder.Decode(req, &scale)
			if err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}

			if err := h.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, oldObj); err != nil {
				if errors.IsNotFound(err) {
					return admission.Errored(http.StatusNotFound, err)
				}
				return admission.Errored(http.StatusBadRequest, err)
			}
			obj = oldObj.DeepCopyObject().(client.Object)
			replicas := scale.Spec.Replicas
			if err := setReplicas(obj, replicas); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}

		sts, err := convToSts(obj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		oldSts, err := convToSts(oldObj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if allErrs := validateStatefulSetUpdate(sts, oldSts); len(allErrs) > 0 {
			return admission.Errored(http.StatusUnprocessableEntity, allErrs.ToAggregate())
		}
	}

	return admission.ValidationResponse(true, "")
}

func convToSts(obj client.Object) (*appsv1.StatefulSet, error) {
	if sts, ok := obj.(*appsv1.StatefulSet); ok {
		return sts, nil
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	sts := &appsv1.StatefulSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u, sts); err != nil {
		return nil, err
	}

	return sts, nil
}

func setReplicas(obj client.Object, replicas int32) error {
	if sts, ok := obj.(*appsv1.StatefulSet); ok {
		sts.Spec.Replicas = &replicas
		return nil
	}
	if asts, ok := obj.(*kruisev1alpha1.StatefulSet); ok {
		asts.Spec.Replicas = &replicas
		return nil
	}
	return fmt.Errorf("unkonw type %v", obj)
}

var _ inject.Client = &StatefulSetCreateUpdateHandler{}

// InjectClient injects the client into the StatefulSetCreateUpdateHandler
func (h *StatefulSetCreateUpdateHandler) InjectClient(c client.Client) error {
	h.Client = c
	return nil
}

var _ admission.DecoderInjector = &StatefulSetCreateUpdateHandler{}

// InjectDecoder injects the decoder into the StatefulSetCreateUpdateHandler
func (h *StatefulSetCreateUpdateHandler) InjectDecoder(d *admission.Decoder) error {
	h.Decoder = d
	return nil
}
