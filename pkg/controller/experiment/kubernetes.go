/*

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

package experiment

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

func (r *ReconcileExperiment) syncKubernetes(context context.Context, instance *iter8v1alpha1.Experiment) (reconcile.Result, error) {
	log := util.Logger(context)

	updateStatus, err := r.checkOrInitRules(context, instance)
	if err != nil {
		if updateStatus {
			if err := r.Status().Update(context, instance); err != nil && !util.ValidUpdateErr(err) {
				log.Info("Fail to update status: %v", err)
				return reconcile.Result{}, nil
			}
		}
		return reconcile.Result{}, nil
	}

	if detectTargets(context, instance) {
		updateStatus, err = r.detectTargets(context, instance)
		if err != nil {
			if updateStatus {
				if err := r.Status().Update(context, instance); err != nil && !util.ValidUpdateErr(err) {
					log.Info("Fail to update status: %v", err)
					return reconcile.Result{}, nil
				}
			}
			return reconcile.Result{}, nil
		}
	}

	hasProgressed := false
	if progress(context, instance) {
		err := r.progressExperiment(context, instance)
		if err := r.Status().Update(context, instance); err != nil && !util.ValidUpdateErr(err) {
			log.Info("Fail to update status: %v", err)
			return reconcile.Result{}, nil
		}

		if err != nil {
			return reconcile.Result{}, nil
		}

		hasProgressed = true
	}

	if completed, err := r.checkExperimentCompleted(context, instance); completed {
		if err := r.Status().Update(context, instance); err != nil && !util.ValidUpdateErr(err) {
			log.Info("Fail to update status: %v", err)
			// End experiment
			return reconcile.Result{}, nil
		}
		// Experiment completed
		return reconcile.Result{}, err
	}

	if hasProgressed {
		traffic := instance.Spec.TrafficControl
		interval, _ := traffic.GetIntervalDuration()
		// Next iteration
		log.Info("Requeue for next iteration")
		return reconcile.Result{RequeueAfter: interval}, nil
	}

	log.Info("Request not processed")
	return reconcile.Result{}, nil
}

func (r *ReconcileExperiment) finalizeIstio(context context.Context, instance *iter8v1alpha1.Experiment) (reconcile.Result, error) {
	completed := instance.Status.GetCondition(iter8v1alpha1.ExperimentConditionExperimentCompleted)
	if completed != nil && completed.Status != corev1.ConditionTrue {
		instance.Action = iter8v1alpha1.ActionOverrideFailure
		if _, err := r.syncKubernetes(context, instance); err != nil {
			util.Logger(context).Error(err, "Fail to execute finalize sync process")
		}
		r.iter8Cache.RemoveExperiment(instance)
	}

	return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
}

func progress(context context.Context, instance *iter8v1alpha1.Experiment) bool {
	if instance.Action.TerminateExperiment() {
		return false
	}

	now := time.Now()
	traffic := instance.Spec.TrafficControl
	interval, _ := traffic.GetIntervalDuration()

	return now.After(instance.Status.LastIncrementTime.Add(interval))
}

func detectTargets(context context.Context, instance *iter8v1alpha1.Experiment) bool {
	if util.ExperimentAbstract(context) != nil && util.ExperimentAbstract(context).Terminate() {
		return false
	}

	cond := instance.Status.GetCondition(iter8v1alpha1.ExperimentConditionTargetsProvided)
	return cond.Status != corev1.ConditionTrue
}
