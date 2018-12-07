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
	"os"
	"os/signal"
	"strings"
	"syscall"

	"code.uplex.de/uplex-varnish/k8s-ingress/cmd/controller"
	"code.uplex.de/uplex-varnish/k8s-ingress/cmd/varnish"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	versionF = flag.Bool("version", false, "print version and exit")
	loglvlF  = flag.String("log-level", "INFO",
		"log level: one of PANIC, FATAL, ERROR, WARN, INFO, DEBUG, \n"+
			"or TRACE")
	logFormat = logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	}
	log = &logrus.Logger{
		Out:       os.Stdout,
		Formatter: &logFormat,
		Level:     logrus.InfoLevel,
	}
)

func main() {
	flag.Parse()

	if *versionF {
		fmt.Printf("%s version %s", os.Args[0], version)
		os.Exit(0)
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

	log.Info("Starting Varnish Ingress controller version:", version)

	var err error
	var config *rest.Config

	config, err = rest.InClusterConfig()
	if err != nil {
		log.Fatalf("error creating client configuration: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Failed to create client:", err)
	}

	vController := varnish.NewVarnishController(log)

	varnishDone := make(chan error, 1)
	vController.Start(varnishDone)

	namespace := os.Getenv("POD_NAMESPACE")
	ingController := controller.NewIngressController(log, kubeClient,
		vController, namespace)
	go handleTermination(log, ingController, vController, varnishDone)
	ingController.Run()
}

func handleTermination(log *logrus.Logger, ingc *controller.IngressController,
	vc *varnish.VarnishController, varnishDone chan error) {

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

	log.Info("Shutting down the ingress controller")
	ingc.Stop()

	if !exited {
		log.Info("Shutting down Varnish")
		vc.Quit()
	}

	log.Info("Exiting with a status:", exitStatus)
	os.Exit(exitStatus)
}
