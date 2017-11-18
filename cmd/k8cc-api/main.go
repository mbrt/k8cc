package main

import (
	"context"
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
)

func main() {
	var (
		httpAddr         = flag.String("http.addr", ":8080", "HTTP listen address")
		namespace        = flag.String("deploy.namespace", "k8cc", "Kubernetes namespace for distcc deployments")
		minReplicas      = flag.Int("autoscale.min-replicas", 1, "Minimum number of replicas with no active users")
		maxReplicas      = flag.Int("autoscale.max-replicas", 10, "Maximum number of replicas")
		replicasPerUser  = flag.Int("autoscale.replicas-per-user", 5, "Number of replicas per active user")
		leaseTimeMinutes = flag.Int("user.lease-time", 15, "Lease time for users in minutes")
		updateSleep      = flag.Int("controller.update-interval", 10, "Update interval of the controller")
	)
	flag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	deployer, err := kube.NewKubeDeployer(*namespace)
	if err != nil {
		_ = logger.Log("err", err)
		os.Exit(1)
	}

	options := controller.AutoScaleOptions{
		MinReplicas:     *minReplicas,
		MaxReplicas:     *maxReplicas,
		ReplicasPerUser: *replicasPerUser,
	}
	leaseTime := time.Duration(*leaseTimeMinutes) * time.Minute

	storage := controller.NewInMemoryStorage()
	controller := controller.NewDeployController(options, leaseTime, deployer, storage, log.With(logger, "component", "controller"))

	var s api.Service
	{
		s = api.NewService(deployer, controller)
		s = api.LoggingMiddleware(logger)(s)
	}

	var h http.Handler
	{
		h = api.MakeHTTPHandler(s, log.With(logger, "component", "HTTP"))
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

	go func() {
		interval := time.Duration(*updateSleep) * time.Second
		ctx := context.Background()

		for {
			time.Sleep(interval)
			controller.DoMaintenance(ctx, time.Now())
		}
	}()

	_ = logger.Log("exit", <-errs)
}
