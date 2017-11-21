package controller

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mbrt/k8cc/pkg/kube"
	kubemock "github.com/mbrt/k8cc/pkg/kube/mock"
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
	storage := NewInMemoryStorage()
	cont := NewStatefulController(opts, storage, deployer, logger).(statefulController)
	tagController := cont.TagController("master")

	// the user comes in
	deployer.EXPECT().ScaleSet(ctx, "master", 3).Return(nil)
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
	assert.Equal(t, 1, storage.NumActiveUsers("master", now))
	currState := []kube.DeploymentState{
		kube.DeploymentState{Tag: "master", Replicas: 3},
	}

	// let's do maintenance now, nothing changes
	deployer.EXPECT().DeploymentsState(ctx).Return(currState, nil)
	cont.DoMaintenance(ctx, now)

	// some times has passed, but the user didn't expire
	now = now.Add(5 * time.Minute)
	deployer.EXPECT().DeploymentsState(ctx).Return(currState, nil)
	cont.DoMaintenance(ctx, now)

	// now the user expired, the set is scaled to 0 replicas
	now = now.Add(6 * time.Minute)
	deployer.EXPECT().DeploymentsState(ctx).Return(currState, nil)
	deployer.EXPECT().ScaleSet(ctx, "master", 0).Return(nil)
	cont.DoMaintenance(ctx, now)

	// FIXME: if a user renew the lease, it'll get new hosts (correct),
	// but then the old hosts are still assigned to the same user. They
	// should instead be revoked
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
	storage := NewInMemoryStorage()
	cont := NewStatefulController(opts, storage, deployer, logger).(statefulController)
	tagController := cont.TagController("master")

	// the first user comes in
	deployer.EXPECT().ScaleSet(ctx, "master", 3).Return(nil)
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
	assert.Equal(t, 1, storage.NumActiveUsers("master", now))
	currState := []kube.DeploymentState{
		kube.DeploymentState{Tag: "master", Replicas: 3},
	}

	// let's do maintenance now, nothing changes
	deployer.EXPECT().DeploymentsState(ctx).Return(currState, nil)
	cont.DoMaintenance(ctx, now)

	// some times has passed, another user arrives
	now = now.Add(5 * time.Minute)
	deployer.EXPECT().ScaleSet(ctx, "master", 5).Return(nil)
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
	assert.Equal(t, expLease, lease)
	assert.Equal(t, 2, storage.NumActiveUsers("master", now))
	currState = []kube.DeploymentState{
		kube.DeploymentState{Tag: "master", Replicas: 5},
	}

	// some more time passes, so the first user expires
	// no scaling is possible, because the second user still holds the last hosts for now
	now = now.Add(5*time.Minute + 1*time.Second)
	deployer.EXPECT().DeploymentsState(ctx).Return(currState, nil)
	cont.DoMaintenance(ctx, now)
	assert.Equal(t, 1, storage.NumActiveUsers("master", now))

	// the first user kicks in again, while the second is still alive
	now = now.Add(1 * time.Minute)
	deployer.EXPECT().ScaleSet(ctx, "master", 5).Return(nil)
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
	assert.Equal(t, expLease, lease)
	assert.Equal(t, 2, storage.NumActiveUsers("master", now))

	// the second user now expires, so the deployment is scaled down
	now = exp2.Add(1 * time.Second)
	deployer.EXPECT().DeploymentsState(ctx).Return(currState, nil)
	deployer.EXPECT().ScaleSet(ctx, "master", 4).Return(nil)
	cont.DoMaintenance(ctx, now)
	assert.Equal(t, 1, storage.NumActiveUsers("master", now))
	currState = []kube.DeploymentState{
		kube.DeploymentState{Tag: "master", Replicas: 4},
	}

	// now also the first expires, so the set replicas goes to 0
	now = exp1.Add(1 * time.Second)
	deployer.EXPECT().DeploymentsState(ctx).Return(currState, nil)
	deployer.EXPECT().ScaleSet(ctx, "master", 0).Return(nil)
	cont.DoMaintenance(ctx, now)
	assert.Equal(t, 0, storage.NumActiveUsers("master", now))
}
