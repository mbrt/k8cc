package controller

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mbrt/k8cc/pkg/data"
	kubemock "github.com/mbrt/k8cc/pkg/kube/mock"
	"github.com/mbrt/k8cc/pkg/state"
)

func TestStatefulSingleUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	operator := kubemock.NewMockOperator(ctrl)

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
	cont := NewStatefulController(opts, tagsstate, operator, logger).(statefulController)
	tagController := cont.TagController("master")

	// the user comes in
	operator.EXPECT().Hostnames(data.Tag("master"), []data.HostID{0, 1, 2}).Return([]string{
		"distcc-0.distcc-master",
		"distcc-1.distcc-master",
		"distcc-2.distcc-master",
	}, nil)
	lease, err := tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 := now.Add(opts.LeaseTime)
	expLease := Lease{
		Expiration: exp1,
		Hosts: []string{
			"distcc-0.distcc-master",
			"distcc-1.distcc-master",
			"distcc-2.distcc-master",
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
	operator.EXPECT().Hostnames(data.Tag("master"), []data.HostID{0, 1, 2}).Return([]string{
		"distcc-0.distcc-master",
		"distcc-1.distcc-master",
		"distcc-2.distcc-master",
	}, nil)
	lease, err = tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 = now.Add(opts.LeaseTime)
	expLease = Lease{
		Expiration: exp1,
		Hosts: []string{
			"distcc-0.distcc-master",
			"distcc-1.distcc-master",
			"distcc-2.distcc-master",
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

	operator := kubemock.NewMockOperator(ctrl)

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
	cont := NewStatefulController(opts, tagsstate, operator, logger).(statefulController)
	tagController := cont.TagController("master")
	mstate := tagsstate.TagState("master")

	// the first user comes in
	operator.EXPECT().Hostnames(data.Tag("master"), []data.HostID{0, 1, 2}).Return([]string{
		"distcc-0.distcc-master",
		"distcc-1.distcc-master",
		"distcc-2.distcc-master",
	}, nil)
	lease, err := tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 := now.Add(opts.LeaseTime)
	expLease := Lease{
		Expiration: exp1,
		Hosts: []string{
			"distcc-0.distcc-master",
			"distcc-1.distcc-master",
			"distcc-2.distcc-master",
		},
	}
	currUsage := []int{1, 1, 1}
	assert.Equal(t, expLease, lease)
	assert.Equal(t, currUsage, mstate.HostsUsage(now))
	assert.Equal(t, 3, tagController.DesiredReplicas(now))

	// some times has passed, another user arrives
	now = now.Add(5 * time.Minute)
	operator.EXPECT().Hostnames(data.Tag("master"), []data.HostID{3, 4, 0}).Return([]string{
		"distcc-3.distcc-master",
		"distcc-4.distcc-master",
		"distcc-0.distcc-master",
	}, nil)
	lease, err = tagController.LeaseUser(ctx, "alice", now)
	assert.Nil(t, err)
	exp2 := now.Add(opts.LeaseTime)
	expLease = Lease{
		Expiration: exp2,
		Hosts: []string{
			"distcc-3.distcc-master",
			"distcc-4.distcc-master",
			"distcc-0.distcc-master",
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
	operator.EXPECT().Hostnames(data.Tag("master"), []data.HostID{1, 2, 3}).Return([]string{
		"distcc-1.distcc-master",
		"distcc-2.distcc-master",
		"distcc-3.distcc-master",
	}, nil)
	lease, err = tagController.LeaseUser(ctx, "mike", now)
	assert.Nil(t, err)
	exp1 = now.Add(opts.LeaseTime)
	expLease = Lease{
		Expiration: exp1,
		Hosts: []string{
			"distcc-1.distcc-master",
			"distcc-2.distcc-master",
			"distcc-3.distcc-master",
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

	// now also the first expires, so the set replicas goes to 0
	now = exp1.Add(1 * time.Second)
	assert.Equal(t, 0, tagController.DesiredReplicas(now))
}
