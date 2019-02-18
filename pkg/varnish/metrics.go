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

package varnish

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "varnishingctl"
	subsystem = "varnish"
)

type instanceMetrics struct {
	updates         prometheus.Counter
	updateErrs      prometheus.Counter
	connectFails    prometheus.Counter
	vclLoads        prometheus.Counter
	vclLoadErrs     prometheus.Counter
	connectLatency  prometheus.Summary
	vclLoadLatency  prometheus.Summary
	pings           prometheus.Counter
	pingFails       prometheus.Counter
	panics          prometheus.Counter
	childRunning    prometheus.Counter
	childNotRunning prometheus.Counter
	vclDiscards     prometheus.Counter
	monitorChecks   prometheus.Counter
}

var (
	svcsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "services",
		Help:      "Current number of managed Varnish services",
	})
	instsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "instances",
		Help:      "Current number of managed Varnish instances",
	})
	secretsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "secrets",
		Help:      "Current number of known admin secrets",
	})

	beSvcsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "backend_services",
		Help: "Current number of Services configured as Varnish " +
			"backends",
	})

	beEndpsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "backend_endpoints",
		Help: "Current number of Service endpoints configured " +
			"as Varnish backends",
	})

	monResultCtr = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "monitor_result_total",
		Help:      "Total number of monitor results",
	}, []string{"service", "status", "result"})

	addr2instMetrics = make(map[string]*instanceMetrics)
	instMetricsMtx   = &sync.Mutex{}

	latencyObjectives = map[float64]float64{
		0.5:   0.001,
		0.9:   0.001,
		0.95:  0.001,
		0.99:  0.001,
		0.999: 0.001,
	}
)

func initMetrics() {
	prometheus.Register(svcsGauge)
	prometheus.Register(instsGauge)
	prometheus.Register(secretsGauge)
	prometheus.Register(monResultCtr)
	prometheus.Register(beSvcsGauge)
	prometheus.Register(beEndpsGauge)
}

func getInstanceMetrics(addr string) *instanceMetrics {
	instMetricsMtx.Lock()
	defer instMetricsMtx.Unlock()

	metrics, exists := addr2instMetrics[addr]
	if exists {
		return metrics
	}
	labels := make(map[string]string)
	labels["varnish_instance"] = addr
	metrics = &instanceMetrics{
		updates: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "updates_total",
			Help:        "Total number of attempted updates",
			ConstLabels: labels,
		}),
		updateErrs: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "update_errors_total",
			Help:        "Total number of update errors",
			ConstLabels: labels,
		}),
		connectFails: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "admin_connect_fails_total",
			Help:        "Total number of admin connection failures",
			ConstLabels: labels,
		}),
		vclLoads: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "vcl_loads_total",
			Help:        "Total number of successful VCL loads",
			ConstLabels: labels,
		}),
		vclLoadErrs: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "vcl_load_errors_total",
			Help:        "Total number of VCL load errors",
			ConstLabels: labels,
		}),
		connectLatency: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "admin_connect_latency_seconds",
			Help:        "Admin connection latency",
			ConstLabels: labels,
			Objectives:  latencyObjectives,
		}),
		vclLoadLatency: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "vcl_load_latency_seconds",
			Help:        "VCL load latency",
			ConstLabels: labels,
			Objectives:  latencyObjectives,
		}),
		pings: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "pings_total",
			Help:        "Total number of successful pings",
			ConstLabels: labels,
		}),
		pingFails: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "ping_errors_total",
			Help:        "Total number of ping errors",
			ConstLabels: labels,
		}),
		panics: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "panics_total",
			Help:        "Total number of panics detected",
			ConstLabels: labels,
		}),
		childRunning: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "child_running_total",
			Help: "Total number of monitor runs with the " +
				"child process in the running state",
			ConstLabels: labels,
		}),
		childNotRunning: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "child_not_running_total",
			Help: "Total number of monitor runs with the " +
				"child process not in the running state",
			ConstLabels: labels,
		}),
		vclDiscards: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "vcl_discards_total",
			Help:        "Total number of VCL discards",
			ConstLabels: labels,
		}),
		monitorChecks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "monitor_checks_total",
			Help:        "Total number of monitor checks",
			ConstLabels: labels,
		}),
	}
	prometheus.Register(metrics.updates)
	prometheus.Register(metrics.updateErrs)
	prometheus.Register(metrics.connectFails)
	prometheus.Register(metrics.vclLoads)
	prometheus.Register(metrics.vclLoadErrs)
	prometheus.Register(metrics.connectLatency)
	prometheus.Register(metrics.vclLoadLatency)
	prometheus.Register(metrics.pings)
	prometheus.Register(metrics.pingFails)
	prometheus.Register(metrics.panics)
	prometheus.Register(metrics.childRunning)
	prometheus.Register(metrics.childNotRunning)
	prometheus.Register(metrics.vclDiscards)
	prometheus.Register(metrics.monitorChecks)
	addr2instMetrics[addr] = metrics
	return metrics
}
