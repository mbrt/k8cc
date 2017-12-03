package controller

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	kubemock "github.com/mbrt/k8cc/pkg/kube/mock"
	"github.com/mbrt/k8cc/pkg/state"
)

func TestStatefulSingleUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := kubemock.NewMockDeployer(ctrl)

	logger := dummyLogger{}
	ctx := context.Background()
	now := time.Now()
	opts := AutoScaleOptions{
		MinReplicas:     1,
		MaxReplicas:     5,
		ReplicasPerUser: 3,
		LeaseTime:       10 * time.Minute,
	}
	tagsstate := state.NewInMemoryState()
	cont := NewStatefulController(opts, tagsstate, deployer, logger).(statefulController)
	tagController := cont.TagController("master")

	// the user comes in
	lease, err := tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 := now.Add(opts.LeaseTime)
	expLease := Lease{
		Expiration: exp1,
		Hosts: []string{
			"k8cc-build-master0",
			"k8cc-build-master1",
			"k8cc-build-master2",
		},
	}
	assert.Equal(t, expLease, lease)
	assert.Equal(t, 3, tagController.DesiredReplicas(now))

	// some times has passed, but the user didn't expire
	now = now.Add(5 * time.Minute)
	assert.Equal(t, 3, tagController.DesiredReplicas(now))

	// if a user renews the lease, it'll get new hosts (correct), but
	// then the old hosts are still assigned to the same user. They
	// should instead be revoked
	lease, err = tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 = now.Add(opts.LeaseTime)
	expLease = Lease{
		Expiration: exp1,
		Hosts: []string{
			"k8cc-build-master0",
			"k8cc-build-master1",
			"k8cc-build-master2",
		},
	}
	assert.Equal(t, expLease, lease)

	// now the user expired, the set is scaled to 0 replicas
	now = exp1.Add(1 * time.Second)
	assert.Equal(t, 0, tagController.DesiredReplicas(now))
}

func TestStatefulTwoUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := kubemock.NewMockDeployer(ctrl)

	logger := dummyLogger{}
	ctx := context.Background()
	now := time.Now()
	opts := AutoScaleOptions{
		MinReplicas:     1,
		MaxReplicas:     5,
		ReplicasPerUser: 3,
		LeaseTime:       10 * time.Minute,
	}
	tagsstate := state.NewInMemoryState()
	cont := NewStatefulController(opts, tagsstate, deployer, logger).(statefulController)
	tagController := cont.TagController("master")
	mstate := tagsstate.TagState("master")

	// the first user comes in
	lease, err := tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 := now.Add(opts.LeaseTime)
	expLease := Lease{
		Expiration: exp1,
		Hosts: []string{
			"k8cc-build-master0",
			"k8cc-build-master1",
			"k8cc-build-master2",
		},
	}
	currUsage := []int{1, 1, 1}
	assert.Equal(t, expLease, lease)
	assert.Equal(t, currUsage, mstate.HostsUsage(now))
	assert.Equal(t, 3, tagController.DesiredReplicas(now))

	// some times has passed, another user arrives
	now = now.Add(5 * time.Minute)
	lease, err = tagController.LeaseUser(ctx, "alice", now)
	assert.Nil(t, err)
	exp2 := now.Add(opts.LeaseTime)
	expLease = Lease{
		Expiration: exp2,
		Hosts: []string{
			"k8cc-build-master3",
			"k8cc-build-master4",
			"k8cc-build-master0",
		},
	}
	currUsage = []int{2, 1, 1, 1, 1}
	assert.Equal(t, expLease, lease)
	assert.Equal(t, currUsage, mstate.HostsUsage(now))
	assert.Equal(t, 5, tagController.DesiredReplicas(now))

	// some more time passes, so the first user expires
	// no scaling is possible, because the second user still holds the last hosts for now
	now = now.Add(5*time.Minute + 1*time.Second)
	currUsage = []int{1, 0, 0, 1, 1}
	assert.Equal(t, currUsage, mstate.HostsUsage(now))
	assert.Equal(t, 5, tagController.DesiredReplicas(now))

	// the first user kicks in again, while the second is still alive
	now = now.Add(1 * time.Minute)
	lease, err = tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 = now.Add(opts.LeaseTime)
	expLease = Lease{
		Expiration: exp1,
		Hosts: []string{
			"k8cc-build-master1",
			"k8cc-build-master2",
			"k8cc-build-master3",
		},
	}
	currUsage = []int{1, 1, 1, 2, 1}
	assert.Equal(t, expLease, lease)
	assert.Equal(t, currUsage, mstate.HostsUsage(now))
	assert.Equal(t, 5, tagController.DesiredReplicas(now))

	// the second user now expires, so the deployment is scaled down
	now = exp2.Add(1 * time.Second)
	currUsage = []int{0, 1, 1, 1}
	assert.Equal(t, currUsage, mstate.HostsUsage(now))
	assert.Equal(t, 4, tagController.DesiredReplicas(now))

	// now also the first expires, so the set replicas goes to 0
	now = exp1.Add(1 * time.Second)
	assert.Equal(t, 0, tagController.DesiredReplicas(now))
}
