/*
 * Copyright (c) 2018 UPLEX Nils Goroll Systemoptimierung
 * All rights reserved
 *
 * Author: Geoffrey Simmons <geoffrey.simmons@uplex.de>
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL AUTHOR OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */

package controller

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

// Don't return error (requeuing the vcfg) if either of Ingresses or
// Services are not found -- they will sync as needed when and if they
// are discovered.
func (worker *NamespaceWorker) enqueueIngsForVcfg(
	vcfg *vcr_v1alpha1.VarnishConfig) error {

	svc2ing := make(map[*api_v1.Service]*extensions.Ingress)
	ings, err := worker.ing.List(labels.Everything())
	if errors.IsNotFound(err) {
		worker.log.Infof("VarnishConfig %s/%s: no Ingresses found in "+
			"workspace %s", vcfg.Namespace, vcfg.Name,
			worker.namespace)
		return nil
	}
	if err != nil {
		return err
	}
	for _, ing := range ings {
		if !worker.isVarnishIngress(ing) {
			continue
		}
		vSvc, err := worker.getVarnishSvcForIng(ing)
		if errors.IsNotFound(err) {
			worker.log.Infof("VarnishConfig %s/%s: no Varnish "+
				"Services found in workspace %s",
				vcfg.Namespace, vcfg.Name, worker.namespace)
			return nil
		}
		if err != nil {
			return err
		}
		if vSvc != nil {
			svc2ing[vSvc] = ing
		}
	}

	svcSet := make(map[string]struct{})
	for _, svc := range vcfg.Spec.Services {
		if _, exists := svcSet[svc]; exists {
			continue
		}
		svcSet[svc] = struct{}{}

		svcObj, err := worker.svc.Get(svc)
		if err != nil {
			return err
		}
		if ing, exists := svc2ing[svcObj]; exists {
			worker.log.Infof("VarnishConfig %s/%s: enqueuing "+
				"Ingress %s/%s for update", vcfg.Namespace,
				vcfg.Name, ing.Namespace, ing.Name)
			worker.queue.Add(&SyncObj{Type: Update, Obj: ing})
		}
	}
	return nil
}

// XXX a validating webhook should do this
func validateRewrites(rewrites []vcr_v1alpha1.RewriteSpec) error {
	for _, rw := range rewrites {
		if rw.Method == vcr_v1alpha1.Delete &&
			strings.HasSuffix(rw.Target, ".url") {

			return fmt.Errorf("target %s may not be deleted",
				rw.Target)
		}
		if rw.Source != "" && (strings.HasPrefix(rw.Target, "be") !=
			strings.HasPrefix(rw.Source, "be")) {

			return fmt.Errorf("target %s and source %s illegally "+
				"mix client and backend contexts", rw.Target,
				rw.Source)
		}
		if rw.Compare != vcr_v1alpha1.Prefix &&
			(rw.Select == vcr_v1alpha1.Exact ||
				rw.Select == vcr_v1alpha1.Longest ||
				rw.Select == vcr_v1alpha1.Shortest) {

			return fmt.Errorf("select value %s not permitted with "+
				"compare value %s", rw.Select, rw.Compare)
		}
		if rw.Compare != vcr_v1alpha1.Match &&
			rw.MatchFlags != nil &&
			((rw.MatchFlags.MaxMem != nil &&
				*rw.MatchFlags.MaxMem != 0) ||
				(rw.MatchFlags.Anchor != "" &&
					rw.MatchFlags.Anchor != vcr_v1alpha1.None) ||
				rw.MatchFlags.UTF8 ||
				rw.MatchFlags.PosixSyntax ||
				rw.MatchFlags.LongestMatch ||
				rw.MatchFlags.Literal ||
				rw.MatchFlags.NeverCapture ||
				rw.MatchFlags.PerlClasses ||
				rw.MatchFlags.WordBoundary) {

			return fmt.Errorf("Only the case-sensitive match flag " +
				"may be set for fixed-string matches")
		}
		// The same Value may not be added in more than one Rule.
		// The Rewrite field is required, unless the method is Delete.
		vals := make(map[string]struct{})
		for _, rule := range rw.Rules {
			if _, exists := vals[rule.Value]; exists {
				return fmt.Errorf("Value \"%s\" appears in "+
					"more than one rule", rule.Value)
			}
			vals[rule.Value] = struct{}{}

			if rw.Method != vcr_v1alpha1.Delete &&
				rule.Rewrite == "" {

				return fmt.Errorf("Rewrite field may not be " +
					"empty, unless the method is delete")
			}
		}
		// XXX what else?
	}
	return nil
}

// XXX validating webhook should do this
func validateReqDisps(reqDisps []vcr_v1alpha1.RequestDispSpec) error {
	for _, disp := range reqDisps {
		if disp.Disposition.Action == vcr_v1alpha1.RecvSynth &&
			disp.Disposition.Status == nil {

			return fmt.Errorf("status not set for request " +
				"disposition synth")
		}
		for _, cond := range disp.Conditions {
			if len(cond.Values) == 0 && cond.Count == nil &&
				cond.Compare != vcr_v1alpha1.Exists &&
				cond.Compare != vcr_v1alpha1.NotExists {
				return fmt.Errorf("no values or count set for "+
					"request disposition condition "+
					"(comparand %s)", cond.Comparand)
			}
			if len(cond.Values) != 0 && cond.Count != nil {
				return fmt.Errorf("both values and count set "+
					"for request disposition condition "+
					"(comparand %s)", cond.Comparand)
			}
			if len(cond.Values) > 0 {
				switch cond.Compare {
				case vcr_v1alpha1.Greater,
					vcr_v1alpha1.GreaterEqual,
					vcr_v1alpha1.Less,
					vcr_v1alpha1.LessEqual:
					return fmt.Errorf("illegal compare "+
						"(comparand %s, compare %s)",
						cond.Comparand, cond.Compare)
				}
				switch cond.Comparand {
				case "req.esi_level", "req.restarts":
					return fmt.Errorf("illegal comparison "+
						"(comparand %s, values %v)",
						cond.Comparand, cond.Values)
				}
			}
			if cond.Count != nil {
				switch cond.Compare {
				case vcr_v1alpha1.Match,
					vcr_v1alpha1.NotMatch,
					vcr_v1alpha1.Prefix,
					vcr_v1alpha1.NotPrefix,
					vcr_v1alpha1.Exists,
					vcr_v1alpha1.NotExists:
					return fmt.Errorf("illegal compare "+
						"(compare %s, count %d)",
						cond.Compare, *cond.Count)
				}
				err := false
				switch cond.Comparand {
				case "req.url", "req.method",
					"req.proto":
					err = true
				}
				if strings.HasPrefix(cond.Comparand, "req.http") {
					err = true
				}
				if err {
					return fmt.Errorf("illegal comparison "+
						"(comparand %s, count %d)",
						cond.Comparand, *cond.Count)
				}
			}
		}
	}
	return nil
}

func (worker *NamespaceWorker) syncVcfg(key string) error {
	worker.log.Infof("Syncing VarnishConfig: %s/%s", worker.namespace, key)
	vcfg, err := worker.vcfg.Get(key)
	if err != nil {
		return err
	}
	worker.log.Tracef("VarnishConfig %s/%s: %+v", vcfg.Namespace,
		vcfg.Name, vcfg)

	if len(vcfg.Spec.Services) == 0 {
		// CRD validation should prevent this.
		worker.log.Infof("VarnishConfig %s/%s: no services defined, "+
			"ignoring", vcfg.Namespace, vcfg.Name)
		syncCounters.WithLabelValues(worker.namespace, "VarnishConfig",
			"Ignore").Inc()
		return nil
	}

	if vcfg.Spec.SelfSharding != nil {
		if err = validateProbe(&vcfg.Spec.SelfSharding.Probe); err != nil {
			return fmt.Errorf("VarnishConfig %s/%s invalid "+
				"sharding spec: %v", vcfg.Namespace, vcfg.Name,
				err)
		}
	}
	if err = validateRewrites(vcfg.Spec.Rewrites); err != nil {
		return err
	}
	if err = validateReqDisps(vcfg.Spec.ReqDispositions); err != nil {
		return err
	}

	return worker.enqueueIngsForVcfg(vcfg)
}

func (worker *NamespaceWorker) addVcfg(key string) error {
	return worker.syncVcfg(key)
}

func (worker *NamespaceWorker) updateVcfg(key string) error {
	return worker.syncVcfg(key)
}

func (worker *NamespaceWorker) deleteVcfg(obj interface{}) error {
	vcfg, ok := obj.(*vcr_v1alpha1.VarnishConfig)
	if !ok || vcfg == nil {
		worker.log.Warnf("Delete VarnishConfig: not found: %v", obj)
		return nil
	}
	worker.log.Infof("Deleting VarnishConfig: %s/%s", vcfg.Namespace,
		vcfg.Name)
	return worker.enqueueIngsForVcfg(vcfg)
}
