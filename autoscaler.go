package k8cc

import "context"

// AutoScaler scales a deployment based on the number of users
type AutoScaler struct {
	opts     AutoScaleOptions
	tag      string
	deployer Deployer
}

// AutoScaleOptions contains options for an AutoScaler
type AutoScaleOptions struct {
	MinReplicas     int
	MaxReplicas     int
	ReplicasPerUser int
}

// NewAutoScaler creates a default autoscaler with the given options
func NewAutoScaler(opts AutoScaleOptions, tag string, d Deployer) AutoScaler {
	return AutoScaler{opts, tag, d}
}

// UpdateUsers scales the number of replicas given the number of users present.
// Returns the new number of replicas.
func (a AutoScaler) UpdateUsers(ctx context.Context, n int) (int, error) {
	r := a.computeReplicas(n)
	err := a.deployer.Scale(ctx, a.tag, r)
	return r, err
}

func (a AutoScaler) computeReplicas(numUsers int) int {
	ideal := numUsers * a.opts.ReplicasPerUser
	switch {
	case ideal < a.opts.MinReplicas:
		return a.opts.MinReplicas
	case ideal > a.opts.MaxReplicas:
		return a.opts.MaxReplicas
	default:
		return ideal
	}
}
