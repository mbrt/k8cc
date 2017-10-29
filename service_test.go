package k8cc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type deployerMock struct {
	ips []net.IP
	err error
}

func (d deployerMock) PodIPs(tag string) ([]net.IP, error) {
	return d.ips, d.err
}

func (d deployerMock) Scale(ctx context.Context, tag string, replicas uint) error {
	return nil
}

func (d deployerMock) DeploymentName(tag string) string {
	return tag
}

func TestHosts(t *testing.T) {
	deployIPs := []net.IP{
		net.ParseIP("10.0.0.5"),
		net.ParseIP("10.0.0.10"),
	}
	deployer := deployerMock{deployIPs, nil}
	service := NewService(deployer)
	ips, err := service.Hosts(context.Background(), "foo")

	expected := make([]Host, len(deployIPs))
	for i, s := range deployIPs {
		expected[i].IP = s
	}

	assert.Nil(t, err)
	assert.Equal(t, expected, ips)
}

type deployerTimeoutMock struct{}

func (d deployerTimeoutMock) PodIPs(tag string) ([]net.IP, error) {
	time.Sleep(300 * time.Millisecond)
	return []net.IP{net.ParseIP("127.0.0.1")}, nil
}

func (d deployerTimeoutMock) Scale(ctx context.Context, tag string, replicas uint) error {
	return nil
}

func (d deployerTimeoutMock) DeploymentName(tag string) string {
	return tag
}

func TestHostsTimeout(t *testing.T) {
	deployer := deployerTimeoutMock{}
	service := NewService(deployer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ips, err := service.Hosts(ctx, "foo")

	assert.Nil(t, ips)
	assert.Equal(t, ErrCanceled, err)
}
