package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"

	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/controller/distcc"
	"github.com/mbrt/k8cc/pkg/controller/distccclaim"
	"github.com/mbrt/k8cc/pkg/controller/distccclientclaim"
)

func main() {
	var (
		kubeConfig    = flag.String("kube.config", "", "Kubeconfig path")
		kubeMasterURL = flag.String("kube.master-url", "", "Kubernetes master URL")
	)
	flag.Parse()

	sharedClient, err := controller.NewSharedClient(*kubeMasterURL, *kubeConfig)
	if err != nil {
		glog.Errorf("error: %s", err)
		os.Exit(1)
	}

	controllers := []controller.Controller{
		distcc.NewController(sharedClient),
		distccclaim.NewController(sharedClient),
		distccclientclaim.NewController(sharedClient),
	}

	errs := make(chan error, 1)
	defer close(errs)
	stopCh := make(chan struct{})

	go func() {
		// handle signals
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		close(stopCh)
		errs <- fmt.Errorf("%s", sig)
	}()

	for _, c := range controllers {
		go func(c controller.Controller) {
			errs <- c.Run(2, stopCh)
		}(c)
	}

	// this last one takes ownership of the main goroutine
	if err = sharedClient.Run(stopCh); err != nil {
		errs <- err
	}

	glog.Errorf("exit: %s", <-errs)
}
