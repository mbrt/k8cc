package controller

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/mbrt/k8cc/pkg/data"
	"github.com/mbrt/k8cc/pkg/kube"
	"github.com/mbrt/k8cc/pkg/state"
)

// NewStatefulController creates a controller that uses StatefulSets to manage
// the build hosts
func NewStatefulController(
	opts AutoScaleOptions,
	tagsState state.TagsStater,
	hostnamer kube.Hostnamer,
	logger log.Logger,
) Controller {
	return statefulController{
		opts,
		tagsState,
		hostnamer,
		logger,
	}
}

type statefulController struct {
	opts      AutoScaleOptions
	tagsState state.TagsStater
	hostnamer kube.Hostnamer
	logger    log.Logger
}

func (c statefulController) TagController(tag data.Tag) TagController {
	return statefulTagController{tag, c.opts, c.tagsState, c.hostnamer}
}

type statefulTagController struct {
	tag       data.Tag
	opts      AutoScaleOptions
	tagsState state.TagsStater
	hostnamer kube.Hostnamer
}

func (c statefulTagController) LeaseUser(ctx context.Context, user data.User, now time.Time) (Lease, error) {
	// remove the possible previous user lease
	c.tagsState.TagState(c.tag).RemoveLease(user)

	// find the free host with the lowest id
	usage := c.tagsState.TagState(c.tag).HostsUsage(now)
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
	hosts := make([]data.HostID, c.opts.ReplicasPerUser)
	for i := range hosts {
		// we have to wrap around max replicas
		hosts[i] = data.HostID(assigned % c.opts.MaxReplicas)
		assigned++
	}

	t := now.Add(c.opts.LeaseTime)
	c.tagsState.TagState(c.tag).SetLease(user, t, hosts)

	hostnames, err := c.hostnamer.Hostnames(c.tag, hosts)
	if err != nil {
		return Lease{}, errors.Wrap(err, "error getting hostnames for build hosts")
	}
	result := Lease{
		Expiration: t,
		Hosts:      hostnames,
	}
	return result, nil
}

func (c statefulTagController) DesiredReplicas(now time.Time) int {
	tagUsage := c.tagsState.TagState(c.tag).HostsUsage(now)
	// find the ID of the last used host in the tag
	hostID := len(tagUsage) - 1
	for ; hostID >= 0; hostID-- {
		if tagUsage[hostID] > 0 {
			break
		}
	}
	// the number of replicas needs to contain the last assigned host
	return hostID + 1
}
