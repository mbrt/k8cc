package k8cc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	mock "github.com/mbrt/k8cc/mock"
	kubemock "github.com/mbrt/k8cc/pkg/kube/mock"
)

func TestServiceHosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployIPs := []net.IP{
		net.ParseIP("10.0.0.5"),
		net.ParseIP("10.0.0.10"),
	}
	deployer := kubemock.NewMockDeployer(ctrl)
	deployer.EXPECT().PodIPs("foo").Return(deployIPs, nil)
	contr := mock.NewMockController(ctrl)

	service := NewService(deployer, contr)
	ips, err := service.Hosts(context.Background(), "foo")

	expected := make([]Host, len(deployIPs))
	for i, s := range deployIPs {
		expected[i].IP = s
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, ips)
}

func TestServiceHostsTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployer := kubemock.NewMockDeployer(ctrl)
	deployer.EXPECT().PodIPs("foo").Do(func(_ string) ([]net.IP, error) {
		time.Sleep(300 * time.Millisecond)
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	})
	contr := mock.NewMockController(ctrl)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	service := NewService(deployer, contr)
	ips, err := service.Hosts(ctx, "foo")

	assert.Nil(t, ips)
	assert.Equal(t, ErrCanceled, err)
}
