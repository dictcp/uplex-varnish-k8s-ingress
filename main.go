package main

//go:generate gogitversion -p main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"code.uplex.de/uplex-varnish/k8s-ingress/controller"
	"code.uplex.de/uplex-varnish/k8s-ingress/varnish"

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
		log.Fatal("error creating client configuration: %v", err)
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
