package controller

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	kubemock "github.com/mbrt/k8cc/pkg/kube/mock"
)

func TestStatefulSingleUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := kubemock.NewMockDeployer(ctrl)

	ctx := context.Background()
	now := time.Now()
	opts := AutoScaleOptions{
		MinReplicas:     1,
		MaxReplicas:     5,
		ReplicasPerUser: 3,
		LeaseTime:       10 * time.Minute,
	}
	storage := NewInMemoryStorage()
	cont := NewStatefulController(opts, storage, deployer).(statefulController)
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

	// let's do maintenance now, nothing changes
	cont.DoMaintenance(ctx, now)
	deployer.EXPECT().ScaleSet(ctx, "master", 3).Return(nil)
	assert.Equal(t, 1, storage.NumActiveUsers("master", now))

	// some times has passed, but the user didn't expire
	now = now.Add(5 * time.Minute)
	deployer.EXPECT().ScaleSet(ctx, "master", 3).Return(nil)
	cont.DoMaintenance(ctx, now)
}
