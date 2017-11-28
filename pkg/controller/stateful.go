package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc/pkg/kube"
)

var (
	kubeServicePrefix = "k8cc-build"
)

// NewStatefulController creates a controller that uses StatefulSets to manage the build hosts
func NewStatefulController(
	opts AutoScaleOptions,
	storage Storage,
	deployer kube.Deployer,
	logger log.Logger,
) Controller {
	return statefulController{
		opts,
		storage,
		deployer,
		logger,
	}
}

type statefulController struct {
	opts     AutoScaleOptions
	storage  Storage
	deployer kube.Deployer
	logger   log.Logger
}

func (c statefulController) DoMaintenance(ctx context.Context, now time.Time) {
	states, err := c.deployer.DeploymentsState(ctx)
	if err != nil {
		_ = c.logger.Log("stage", "maintenance", "err", err)
	}
	for _, ds := range states {
		logger := log.With(c.logger, "stage", "maintenance", "tag", ds.Tag)
		tagUsage := c.storage.Usage(ds.Tag, now)
		// find the ID of the last used host in the tag
		hostID := len(tagUsage) - 1
		for ; hostID >= 0; hostID-- {
			if tagUsage[hostID] > 0 {
				break
			}
		}
		// scale back the replicas if they are unused
		requiredReplicas := hostID + 1
		if requiredReplicas != ds.Replicas {
			_ = logger.Log("replicas", requiredReplicas)
			if err = c.deployer.ScaleSet(ctx, ds.Tag, requiredReplicas); err != nil {
				_ = logger.Log("err", err)
			}
		}
	}
}

func (c statefulController) TagController(tag string) TagController {
	return statefulTagController{tag, c.opts, c.storage, c.deployer}
}

type statefulTagController struct {
	tag      string
	opts     AutoScaleOptions
	storage  Storage
	deployer kube.Deployer
}

func (c statefulTagController) LeaseUser(ctx context.Context, user string, now time.Time) (Lease, error) {
	// remove the possible previous user lease
	c.storage.RemoveLease(c.tag, user)

	// find the free host with the lowest id
	usage := c.storage.Usage(c.tag, now)
	assigned := len(usage) // default to the last (non-assigned) one
	if len(usage) > 0 {
		minUsage := usage[0]
		for id, n := range usage {
			if n < minUsage {
				minUsage = n
				assigned = id
			}
		}
	}
	hosts := make([]BuildHostID, c.opts.ReplicasPerUser)
	for i := range hosts {
		// we have to wrap around max replicas
		hosts[i] = BuildHostID(assigned % c.opts.MaxReplicas)
		assigned++
	}

	t := now.Add(c.opts.LeaseTime)
	c.storage.SetLease(c.tag, user, t, hosts)

	hostnames := make([]string, len(hosts))
	for i, id := range hosts {
		hostnames[i] = fmt.Sprintf("%s-%s%d", kubeServicePrefix, c.tag, id)
	}

	nactive := c.storage.NumActiveUsers(c.tag, now)
	replicas := c.computeReplicas(nactive)
	err := c.deployer.ScaleSet(ctx, c.tag, replicas)

	result := Lease{
		Expiration: t,
		Hosts:      hostnames,
	}
	return result, err
}

func (c *statefulTagController) computeReplicas(numUsers int) int {
	ideal := numUsers * c.opts.ReplicasPerUser
	switch {
	case ideal < c.opts.MinReplicas:
		return c.opts.MinReplicas
	case ideal > c.opts.MaxReplicas:
		return c.opts.MaxReplicas
	default:
		return ideal
	}
}
