package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc/pkg/api"
	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/kube"
	"github.com/mbrt/k8cc/pkg/state"
)

func main() {
	var (
		httpAddr         = flag.String("http.addr", ":8080", "HTTP listen address")
		minReplicas      = flag.Int("scale.min-replicas", 1, "Minimum number of replicas with no active users")
		maxReplicas      = flag.Int("scale.max-replicas", 10, "Maximum number of replicas")
		replicasPerUser  = flag.Int("scale.replicas-per-user", 5, "Number of replicas per active user")
		leaseTimeMinutes = flag.Int("user.lease-time", 15, "Lease time for users in minutes")
		kubeConfig       = flag.String("kube.config", "", "Kubeconfig path")
		kubeMasterURL    = flag.String("kube.master-url", "", "Kubernetes master URL")
	)
	flag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	options := controller.AutoScaleOptions{
		MinReplicas:     *minReplicas,
		MaxReplicas:     *maxReplicas,
		ReplicasPerUser: *replicasPerUser,
		LeaseTime:       time.Duration(*leaseTimeMinutes) * time.Minute,
	}

	adapter := &controller.Adapter{}
	tagstate := state.NewInMemoryState()
	contr := controller.NewStatefulController(options, tagstate, adapter, log.With(logger, "component", "controller"))

	operator, err := kube.NewOperator(*kubeMasterURL, *kubeConfig, adapter)
	if err != nil {
		/* #nosec */
		_ = logger.Log("err", err)
		os.Exit(1)
	}

	// set now the objects for the adapter
	adapter.Controller = contr
	adapter.Operator = operator

	var s api.Service
	{
		s = api.NewService(contr, operator)
		s = api.LoggingMiddleware(logger)(s)
	}

	var h http.Handler
	{
		h = api.MakeHTTPHandler(s, log.With(logger, "component", "HTTP"))
	}

	errs := make(chan error)
	defer close(errs)

	go func() {
		c := make(chan struct{})
		errs <- operator.Run(2, c)
	}()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	go func() {
		/* #nosec */
		_ = logger.Log("transport", "HTTP", "addr", *httpAddr)
		errs <- http.ListenAndServe(*httpAddr, h)
	}()

	/* #nosec */
	_ = logger.Log("exit", <-errs)
}
