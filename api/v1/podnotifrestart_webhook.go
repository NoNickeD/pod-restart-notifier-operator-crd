/*
Copyright 2023.

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

package v1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var podnotifrestartlog = logf.Log.WithName("podnotifrestart-resource")

func (r *PodNotifRestart) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-monitoring-vodafone-com-v1-podnotifrestart,mutating=true,failurePolicy=fail,sideEffects=None,groups=monitoring.vodafone.com,resources=podnotifrestarts,verbs=create;update,versions=v1,name=mpodnotifrestart.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &PodNotifRestart{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *PodNotifRestart) Default() {
	podnotifrestartlog.Info("default", "name", r.Name)

	if r.Spec.MinRestarts == 0 {
		r.Spec.MinRestarts = 1
	}
}

//+kubebuilder:webhook:path=/validate-monitoring-vodafone-com-v1-podnotifrestart,mutating=false,failurePolicy=fail,sideEffects=None,groups=monitoring.vodafone.com,resources=podnotifrestarts,verbs=create;update,versions=v1,name=vpodnotifrestart.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &PodNotifRestart{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *PodNotifRestart) ValidateCreate() (admission.Warnings, error) {
	podnotifrestartlog.Info("validate create", "name", r.Name)

	return nil, r.ValidateWebhook()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *PodNotifRestart) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	podnotifrestartlog.Info("validate update", "name", r.Name)

	return nil, r.ValidateWebhook()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *PodNotifRestart) ValidateDelete() (admission.Warnings, error) {
	podnotifrestartlog.Info("validate delete", "name", r.Name)
	// Nothing to validate on delete, allow it.
	return nil, nil
}

func (r *PodNotifRestart) ValidateWebhook() error {
	if !r.Validate() {
		return fmt.Errorf("at least one webhook URL should be specified")
	}

	return nil
}
