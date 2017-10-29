package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc"
)

func main() {
	var (
		httpAddr  = flag.String("http.addr", ":8080", "HTTP listen address")
		namespace = flag.String("deploy.namespace", "k8cc", "Kubernetes namespace for distcc deployments")
	)
	flag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	deployer, err := k8cc.NewKubeDeployer(*namespace)
	if err != nil {
		_ = logger.Log("err", err)
		os.Exit(1)
	}

	var s k8cc.Service
	{
		s = k8cc.NewService(deployer)
		s = k8cc.LoggingMiddleware(logger)(s)
	}

	var h http.Handler
	{
		h = k8cc.MakeHTTPHandler(s, log.With(logger, "component", "HTTP"))
	}

	errs := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	go func() {
		_ = logger.Log("transport", "HTTP", "addr", *httpAddr)
		errs <- http.ListenAndServe(*httpAddr, h)
	}()

	_ = logger.Log("exit", <-errs)
}
