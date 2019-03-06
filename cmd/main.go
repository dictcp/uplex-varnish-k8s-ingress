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

package main

//go:generate gogitversion -p main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	clientset "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/clientset/versioned"
	vcr_informers "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/informers/externalversions"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/controller"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish"

	"github.com/sirupsen/logrus"

	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	versionF = flag.Bool("version", false, "print version and exit")
	loglvlF  = flag.String("log-level", "INFO",
		"log level: one of PANIC, FATAL, ERROR, WARN, INFO, DEBUG, \n"+
			"or TRACE")
	namespaceF = flag.String("namespace", api_v1.NamespaceAll,
		"namespace in which to listen for resources (default all)")
	tmplDirF = flag.String("templatedir", "",
		"directory of templates for VCL generation. Defaults to \n"+
			"the TEMPLATE_DIR env variable, if set, or the \n"+
			"current working directory when the ingress \n"+
			"controller is invoked")
	masterURLF = flag.String("masterurl", "", "cluster master URL, for "+
		"out-of-cluster runs")
	kubeconfigF = flag.String("kubeconfig", "", "config path for the "+
		"cluster master URL, for out-of-cluster runs")
	readyfileF = flag.String("readyfile", "", "path of a file to touch "+
		"when the controller is ready,\nfor readiness probes")
	monIntvlF = flag.Duration("monitorintvl", 30*time.Second,
		"interval at which the monitor thread checks and updates\n"+
			"instances of Varnish that implement Ingress.\n"+
			"Monitor deactivated when <= 0s")
	metricsPortF = flag.Uint("metricsport", 8080,
		"port at which to listen for the /metrics endpoint")
	ingressClassF = flag.String("class", "varnish", "value of the Ingress "+
		"annotation kubernetes.io/ingress.class\nthe controller only "+
		"considers Ingresses with this value for the\nannotation")
	logFormat = logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	}
	log = &logrus.Logger{
		Out:       os.Stdout,
		Formatter: &logFormat,
		Level:     logrus.InfoLevel,
	}
	informerStop = make(chan struct{})
)

const resyncPeriod = 0

//	resyncPeriod    = 30 * time.Second

// Satisifes type TweakListOptionsFunc in
// k8s.io/client-go/informers/internalinterfaces, for use in
// NewFilteredSharedInformerFactory below.
func noop(opts *meta_v1.ListOptions) {}

func main() {
	flag.Parse()

	if *versionF {
		fmt.Printf("%s version %s\n", os.Args[0], version)
		os.Exit(0)
	}

	if *readyfileF != "" {
		if err := os.Remove(*readyfileF); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Cannot remove ready file %s: %v",
				*readyfileF, err)
			os.Exit(-1)
		}
	}

	lvl := strings.ToLower(*loglvlF)
	switch lvl {
	case "panic":
		log.Level = logrus.PanicLevel
	case "fatal":
		log.Level = logrus.FatalLevel
	case "error":
		log.Level = logrus.ErrorLevel
	case "warn":
		log.Level = logrus.WarnLevel
	case "debug":
		log.Level = logrus.DebugLevel
	case "trace":
		log.Level = logrus.TraceLevel
	case "info":
		break
	default:
		fmt.Printf("Unknown log level %s, exiting", *loglvlF)
		os.Exit(-1)
	}

	if *ingressClassF == "" {
		log.Fatalf("class may not be empty")
		os.Exit(-1)
	}

	if *metricsPortF > math.MaxUint16 {
		log.Fatalf("metricsport %d out of range (max %d)",
			*metricsPortF, math.MaxUint16)
		os.Exit(-1)
	}

	log.Info("Starting Varnish Ingress controller version:", version)
	log.Info("Ingress class:", *ingressClassF)

	vController, err := varnish.NewVarnishController(log, *tmplDirF,
		*monIntvlF)
	if err != nil {
		log.Fatal("Cannot initialize Varnish controller: ", err)
		os.Exit(-1)
	}

	config, err := clientcmd.BuildConfigFromFlags(*masterURLF, *kubeconfigF)
	if err != nil {
		log.Fatalf("error creating client configuration: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Failed to create client:", err)
	}
	vingClient, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatal("Failed to create client:", err)
	}

	var informerFactory informers.SharedInformerFactory
	var vcrInformerFactory vcr_informers.SharedInformerFactory
	if *namespaceF == api_v1.NamespaceAll {
		informerFactory = informers.NewSharedInformerFactory(
			kubeClient, resyncPeriod)
		vcrInformerFactory = vcr_informers.NewSharedInformerFactory(
			vingClient, resyncPeriod)
	} else {
		informerFactory = informers.NewFilteredSharedInformerFactory(
			kubeClient, resyncPeriod, *namespaceF, noop)
		vcrInformerFactory =
			vcr_informers.NewFilteredSharedInformerFactory(
				vingClient, resyncPeriod, *namespaceF, noop)

		// XXX this is prefered, but only available in newer
		// versions of client-go.
		// informerFactory = informers.NewSharedInformerFactoryWithOptions(
		// 	kubeClient, resyncPeriod,
		// 	informers.WithNamespace(*namespaceF))
	}

	ingController, err := controller.NewIngressController(log,
		*ingressClassF, kubeClient, vController, informerFactory,
		vcrInformerFactory)
	if err != nil {
		log.Fatalf("Could not initialize controller: %v")
		os.Exit(-1)
	}
	vController.EvtGenerator(ingController)
	varnishDone := make(chan error, 1)
	go handleTermination(log, ingController, vController, varnishDone)
	vController.Start(varnishDone)
	informerFactory.Start(informerStop)
	ingController.Run(*readyfileF, uint16(*metricsPortF))
}

func handleTermination(
	log *logrus.Logger,
	ingc *controller.IngressController,
	vc *varnish.VarnishController,
	varnishDone chan error) {

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)

	exitStatus := 0
	exited := false

	select {
	case err := <-varnishDone:
		if err != nil {
			log.Error("varnish controller exited with an error:",
				err)
			exitStatus = 1
		} else {
			log.Info("varnish controller exited successfully")
		}
		exited = true
	case <-signalChan:
		log.Info("Received SIGTERM, shutting down")
	}

	log.Info("Shutting down informers")
	informerStop <- struct{}{}

	log.Info("Shutting down the ingress controller")
	ingc.Stop()

	if !exited {
		log.Info("Shutting down the Varnish controller")
		vc.Quit()
	}

	log.Info("Exiting with a status:", exitStatus)
	os.Exit(exitStatus)
}
