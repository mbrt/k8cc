package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/controller/distcc"
	"github.com/mbrt/k8cc/pkg/controller/distccclientclaim"
)

func main() {
	var (
		kubeConfig    = flag.String("kube.config", "", "Kubeconfig path")
		kubeMasterURL = flag.String("kube.master-url", "", "Kubernetes master URL")
	)
	flag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	sharedClient, err := controller.NewSharedClient(*kubeMasterURL, *kubeConfig)
	if err != nil {
		/* #nosec */
		_ = logger.Log("err", err)
		os.Exit(1)
	}

	controllers := []controller.Controller{
		distccclient.NewController(sharedClient),
		distcc.NewController(sharedClient),
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

	/* #nosec */
	_ = logger.Log("exit", <-errs)
}
