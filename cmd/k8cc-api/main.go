package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc/pkg/algo"
	"github.com/mbrt/k8cc/pkg/controller"
	clientctrl "github.com/mbrt/k8cc/pkg/controller/client"
	"github.com/mbrt/k8cc/pkg/controller/distcc"
	"github.com/mbrt/k8cc/pkg/service"
	"github.com/mbrt/k8cc/pkg/state"
)

func main() {
	var (
		httpAddr      = flag.String("http.addr", ":8080", "HTTP listen address")
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

	adapter := &algo.Adapter{}
	tagstate := state.NewInMemoryState()
	contr := algo.NewStatefulController(adapter, tagstate, adapter, log.With(logger, "component", "controller"))

	sharedClient, err := controller.NewSharedClient(*kubeMasterURL, *kubeConfig)
	if err != nil {
		/* #nosec */
		_ = logger.Log("err", err)
		os.Exit(1)
	}

	// load the state before to start anything
	if err = state.LoadFrom(tagstate, distcc.NewStateLoader(sharedClient)); err != nil {
		/* #nosec */
		_ = logger.Log("err", err)
		os.Exit(1)
	}

	clientController := clientctrl.NewController(sharedClient, log.With(logger, "component", "client-controller"))

	operator := distcc.NewOperator(sharedClient, adapter, log.With(logger, "component", "operator"))

	// set now the objects for the adapter
	adapter.Controller = contr
	adapter.Operator = operator
	adapter.State = tagstate

	var s service.Service
	{
		s = service.NewService(contr, operator)
		s = service.LoggingMiddleware(logger)(s)
	}

	var h http.Handler
	{
		h = service.MakeHTTPHandler(s, log.With(logger, "component", "HTTP"))
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

	go func() {
		errs <- sharedClient.Run(stopCh)
	}()

	go func() {
		/* #nosec */
		_ = logger.Log("transport", "HTTP", "addr", *httpAddr)
		errs <- http.ListenAndServe(*httpAddr, h)
	}()

	go func() {
		errs <- clientController.Run(2, stopCh)
	}()

	// this last one takes ownership of the main goroutine
	if err = operator.Run(2, stopCh); err != nil {
		errs <- err
	}

	/* #nosec */
	_ = logger.Log("exit", <-errs)
}
