package distccold

import (
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	"github.com/mbrt/k8cc/pkg/data"

	"github.com/mbrt/k8cc/pkg/controller"
)

// StateLoader loads the user leases from all the Distcc objects
type StateLoader struct {
	k8ccclientset clientset.Interface
}

// NewStateLoader makes a new StateLoader starting from an already connected client
func NewStateLoader(client *controller.SharedClient) StateLoader {
	return StateLoader{
		k8ccclientset: client.K8ccClientset,
	}
}

// Load implements the state.Loader interface
func (s StateLoader) Load() ([]data.TagLeases, error) {
	// NOTE: This doesn't use the informer pattern because it's just a big hastle
	// in the loading phase. Since we do this only once, at the beginning, it's
	// perfectly justified.
	//
	// List all distccs across all namespaces
	distccs, err := s.k8ccclientset.K8ccV1alpha1().Distccs("").List(v1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list distccs")
	}
	res := make([]data.TagLeases, len(distccs.Items))
	for i, distcc := range distccs.Items {
		leases := make([]data.Lease, len(distcc.Status.Leases))
		for i, lease := range distcc.Status.Leases {
			leases[i] = toLease(lease)
		}
		res[i].Tag = data.Tag{Namespace: distcc.Namespace, Name: distcc.Name}
		res[i].Leases = leases
	}
	return res, nil
}

func toHostIDs(dh []int32) []data.HostID {
	res := make([]data.HostID, len(dh))
	for i, h := range dh {
		res[i] = data.HostID(h)
	}
	return res
}

func toLease(dl k8ccv1alpha1.DistccLease) data.Lease {
	return data.Lease{
		User:       data.User(dl.UserName),
		Expiration: dl.ExpirationTime.Time,
		Hosts:      toHostIDs(dl.AssignedOrdinals),
	}
}
