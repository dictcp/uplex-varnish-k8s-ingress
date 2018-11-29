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
	"log"
	"os"
	"os/signal"
	"syscall"

	"code.uplex.de/uplex-varnish/k8s-ingress/cmd/controller"
	"code.uplex.de/uplex-varnish/k8s-ingress/cmd/varnish"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var versionF = flag.Bool("version", false, "print version and exit")

func main() {
	flag.Parse()

	if *versionF {
		fmt.Printf("%s version %s", os.Args[0], version)
		os.Exit(0)
	}

	log.Print("Starting Varnish Ingress controller version:", version)

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

	vController := varnish.NewVarnishController()

	varnishDone := make(chan error, 1)
	vController.Start(varnishDone)

	namespace := os.Getenv("POD_NAMESPACE")
	ingController := controller.NewIngressController(kubeClient,
		vController, namespace)
	go handleTermination(ingController, vController, varnishDone)
	ingController.Run()
}

func handleTermination(ingc *controller.IngressController,
	vc *varnish.VarnishController, varnishDone chan error) {

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)

	exitStatus := 0
	exited := false

	select {
	case err := <-varnishDone:
		if err != nil {
			log.Print("varnish controller exited with an error:",
				err)
			exitStatus = 1
		} else {
			log.Print("varnish controller exited successfully")
		}
		exited = true
	case <-signalChan:
		log.Print("Received SIGTERM, shutting down")
	}

	log.Print("Shutting down the ingress controller")
	ingc.Stop()

	if !exited {
		log.Print("Shutting down Varnish")
		vc.Quit()
		<-varnishDone
	}

	log.Print("Exiting with a status:", exitStatus)
	os.Exit(exitStatus)
}
