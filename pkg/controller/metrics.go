/*
 * Copyright (c) 2019 UPLEX Nils Goroll Systemoptimierung
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
	"net/http"

	"k8s.io/client-go/util/workqueue"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace      = "varnishingctl"
	workqSubsystem = "workqueue"
)

type promProvider struct{}

func (_ promProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	label := make(map[string]string)
	label["namespace"] = name
	depth := prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem:   workqSubsystem,
		Namespace:   namespace,
		Name:        "depth",
		Help:        "Current depth of the workqueue",
		ConstLabels: label,
	})
	prometheus.Register(depth)
	return depth
}

func (_ promProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	label := make(map[string]string)
	label["namespace"] = name
	adds := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem:   workqSubsystem,
		Namespace:   namespace,
		Name:        "adds_total",
		Help:        "Total number of adds handled by the workqueue",
		ConstLabels: label,
	})
	prometheus.Register(adds)
	return adds
}

func (_ promProvider) NewLatencyMetric(name string) workqueue.SummaryMetric {
	label := make(map[string]string)
	label["namespace"] = name
	latency := prometheus.NewSummary(prometheus.SummaryOpts{
		Subsystem: workqSubsystem,
		Namespace: namespace,
		Name:      "latency_useconds",
		Help: "Time spent (in µsecs) by items waiting in the " +
			"workqueue",
		ConstLabels: label,
	})
	prometheus.Register(latency)
	return latency
}

func (_ promProvider) NewWorkDurationMetric(name string) workqueue.SummaryMetric {
	label := make(map[string]string)
	label["namespace"] = name
	workDuration := prometheus.NewSummary(prometheus.SummaryOpts{
		Subsystem: workqSubsystem,
		Namespace: namespace,
		Name:      "work_duration_useconds",
		Help: "Time needed (in µsecs) to process items from the " +
			"workqueue",
		ConstLabels: label,
	})
	prometheus.Register(workDuration)
	return workDuration
}

func (_ promProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	label := make(map[string]string)
	label["namespace"] = name
	retries := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem:   workqSubsystem,
		Namespace:   namespace,
		Name:        "retries_total",
		Help:        "Total number of retries handled by workqueue",
		ConstLabels: label,
	})
	prometheus.Register(retries)
	return retries
}

var (
	watchCounters = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "watcher",
		Namespace: namespace,
		Name:      "events_total",
		Help:      "Total number of watcher API events",
	}, []string{"kind", "event"})

	syncCounters = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "sync",
		Name:      "result_total",
		Help:      "Total number of synchronization results",
	}, []string{"namespace", "kind", "result"})
)

func InitMetrics() {
	workqueue.SetProvider(promProvider{})
	prometheus.Register(watchCounters)
	prometheus.Register(syncCounters)
}

func ServeMetrics(log *logrus.Logger, port uint16) {
	addr := fmt.Sprintf(":%d", port)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(addr, nil))
}
