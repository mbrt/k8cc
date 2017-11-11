package controller

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	kubemock "github.com/mbrt/k8cc/pkg/kube/mock"
)

var (
	logger = dummyLogger{}
)

func TestControllerSingleUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := kubemock.NewMockDeployer(ctrl)

	ctx := context.Background()
	now := time.Now()
	leaseTime := 10 * time.Minute
	opts := AutoScaleOptions{
		MinReplicas:     1,
		MaxReplicas:     5,
		ReplicasPerUser: 3,
	}
	storage := NewInMemoryStorage()
	cont := NewController(opts, leaseTime, deployer, storage, logger).(*controller)

	// the user comes in
	cont.LeaseUser("mike", "master", now)

	// test it is considered as active now
	assert.Equal(t, 1, storage.NumActiveUsers("master", now))

	// let's do maintenance now
	deployer.EXPECT().ScaleDeploy(gomock.Any(), "master", 3).Return(nil)
	cont.DoMaintenance(ctx, now)

	// some times has passed, but the user didn't expire
	now = now.Add(5 * time.Minute)
	deployer.EXPECT().ScaleDeploy(gomock.Any(), "master", 3).Return(nil)
	cont.DoMaintenance(ctx, now)

	// renew the lease for the same user
	cont.LeaseUser("mike", "master", now)

	// now if other 6 minutes passed, the lease shouldn't have expired
	now = now.Add(6 * time.Minute)
	deployer.EXPECT().ScaleDeploy(gomock.Any(), "master", 3).Return(nil)
	cont.DoMaintenance(ctx, now)

	// and now if other 5 pass, it should expire
	now = now.Add(5 * time.Minute)
	deployer.EXPECT().ScaleDeploy(gomock.Any(), "master", 1).Return(nil)
	cont.DoMaintenance(ctx, now)
}

func TestControllerTwoUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := kubemock.NewMockDeployer(ctrl)

	ctx := context.Background()
	now := time.Now()
	leaseTime := 10 * time.Minute
	opts := AutoScaleOptions{
		MinReplicas:     1,
		MaxReplicas:     5,
		ReplicasPerUser: 3,
	}
	storage := NewInMemoryStorage()
	controller := NewController(opts, leaseTime, deployer, storage, logger)

	// the user comes in
	controller.LeaseUser("mike", "master", now)

	// let's do maintenance now
	deployer.EXPECT().ScaleDeploy(gomock.Any(), "master", 3).Return(nil)
	controller.DoMaintenance(ctx, now)

	// after 3 minutes another user arrives
	now = now.Add(3 * time.Minute)
	controller.LeaseUser("alice", "master", now)

	// maximum deployments has been reached
	deployer.EXPECT().ScaleDeploy(gomock.Any(), "master", 5).Return(nil)
	controller.DoMaintenance(ctx, now)

	// now 8 minutes pass: the first user expires, the second doesn't
	now = now.Add(8 * time.Minute)
	deployer.EXPECT().ScaleDeploy(gomock.Any(), "master", 3).Return(nil)
	controller.DoMaintenance(ctx, now)
}

type dummyLogger struct{}

func (d dummyLogger) Log(keyvals ...interface{}) error {
	return nil
}
