package k8cc

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	mock "github.com/mbrt/k8cc/mock"
)

var (
	logger = dummyLogger{}
)

func TestControllerSingleUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := mock.NewMockDeployer(ctrl)
	clock := mock.NewMockClock(ctrl)

	ctx := context.Background()
	now := time.Now()
	leaseTime := 10 * time.Minute
	opts := AutoScaleOptions{
		MinReplicas:     1,
		MaxReplicas:     5,
		ReplicasPerUser: 3,
	}
	cont := NewController(opts, leaseTime, deployer, clock, logger).(*controller)

	// the user comes in
	clock.EXPECT().Now().Return(now)
	cont.LeaseUser("mike", "master")

	// test it is considered as active now
	clock.EXPECT().Now().Return(now)
	assert.Equal(t, 1, cont.tagControllers["master"].uac.ActiveUsers())

	// let's do maintenance now
	clock.EXPECT().Now().Return(now)
	deployer.EXPECT().Scale(gomock.Any(), "master", 3).Return(nil)
	cont.DoMaintenance(ctx)

	// some times has passed, but the user didn't expire
	now = now.Add(5 * time.Minute)
	clock.EXPECT().Now().Return(now)
	deployer.EXPECT().Scale(gomock.Any(), "master", 3).Return(nil)
	cont.DoMaintenance(ctx)

	// renew the lease for the same user
	clock.EXPECT().Now().Return(now)
	cont.LeaseUser("mike", "master")

	// now if other 6 minutes passed, the lease shouldn't have expired
	now = now.Add(6 * time.Minute)
	clock.EXPECT().Now().Return(now)
	deployer.EXPECT().Scale(gomock.Any(), "master", 3).Return(nil)
	cont.DoMaintenance(ctx)

	// and now if other 5 pass, it should expire
	now = now.Add(5 * time.Minute)
	clock.EXPECT().Now().Return(now)
	deployer.EXPECT().Scale(gomock.Any(), "master", 1).Return(nil)
	cont.DoMaintenance(ctx)
}

func TestControllerTwoUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := mock.NewMockDeployer(ctrl)
	clock := mock.NewMockClock(ctrl)

	ctx := context.Background()
	now := time.Now()
	leaseTime := 10 * time.Minute
	opts := AutoScaleOptions{
		MinReplicas:     1,
		MaxReplicas:     5,
		ReplicasPerUser: 3,
	}
	controller := NewController(opts, leaseTime, deployer, clock, logger)

	// the user comes in
	clock.EXPECT().Now().Return(now)
	controller.LeaseUser("mike", "master")

	// let's do maintenance now
	clock.EXPECT().Now().Return(now)
	deployer.EXPECT().Scale(gomock.Any(), "master", 3).Return(nil)
	controller.DoMaintenance(ctx)

	// after 3 minutes another user arrives
	now = now.Add(3 * time.Minute)
	clock.EXPECT().Now().Return(now)
	controller.LeaseUser("alice", "master")

	// maximum deployments has been reached
	clock.EXPECT().Now().Return(now)
	deployer.EXPECT().Scale(gomock.Any(), "master", 5).Return(nil)
	controller.DoMaintenance(ctx)

	// now 8 minutes pass: the first user expires, the second doesn't
	now = now.Add(8 * time.Minute)
	clock.EXPECT().Now().Return(now)
	deployer.EXPECT().Scale(gomock.Any(), "master", 3).Return(nil)
	controller.DoMaintenance(ctx)
}

type dummyLogger struct{}

func (d dummyLogger) Log(keyvals ...interface{}) error {
	return nil
}
