/*
Copyright 2023 The Kubernetes Authors.

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

package rules

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/core/scaledown/pdb"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/daemonset"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/localstorage"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/longterminating"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/mirror"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/notsafetoevict"
	pdbrule "k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/pdb"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/replicacount"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/replicated"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/safetoevict"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/system"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/drainability/rules/terminal"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/options"
)

// Rule determines whether a given pod can be drained or not.
type Rule interface {
	// Drainable determines whether a given pod is drainable according to
	// the specific Rule.
	//
	// DrainContext cannot be nil.
	Drainable(*drainability.DrainContext, *apiv1.Pod) drainability.Status
}

// Default returns the default list of Rules.
func Default(deleteOptions options.NodeDeleteOptions) Rules {
	var rules Rules
	for _, r := range []struct {
		rule Rule
		skip bool
	}{
		{rule: mirror.New()},
		{rule: longterminating.New()},
		{rule: replicacount.New(deleteOptions.MinReplicaCount), skip: !deleteOptions.SkipNodesWithCustomControllerPods},

		// Interrupting checks
		{rule: daemonset.New()},
		{rule: safetoevict.New()},
		{rule: terminal.New()},

		// Blocking checks
		{rule: replicated.New(deleteOptions.SkipNodesWithCustomControllerPods)},
		{rule: system.New(), skip: !deleteOptions.SkipNodesWithSystemPods},
		{rule: notsafetoevict.New()},
		{rule: localstorage.New(), skip: !deleteOptions.SkipNodesWithLocalStorage},
		{rule: pdbrule.New()},
	} {
		if !r.skip {
			rules = append(rules, r.rule)
		}
	}
	return rules
}

// Rules defines operations on a collections of rules.
type Rules []Rule

// Drainable determines whether a given pod is drainable according to the
// specified set of rules.
func (rs Rules) Drainable(drainCtx *drainability.DrainContext, pod *apiv1.Pod) drainability.Status {
	if drainCtx == nil {
		drainCtx = &drainability.DrainContext{}
	}
	if drainCtx.RemainingPdbTracker == nil {
		drainCtx.RemainingPdbTracker = pdb.NewBasicRemainingPdbTracker()
	}

	for _, r := range rs {
		d := r.Drainable(drainCtx, pod)
		if d.Outcome != drainability.UndefinedOutcome {
			return d
		}
	}
	return drainability.NewUndefinedStatus()
}
