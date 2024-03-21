package sync

import "errors"

type Datacenter struct {
	ID          string
	Name        string
	Description string
}

type Cluster struct {
	ID          string
	Name        string
	Description string
}

type VM struct {
	ID          string
	Name        string
	Description string
	Memory      int
	Diskspace   int
	Network     []NIC
	Status      string
}

type NIC struct {
	ID          string
	Name        string
	MAC         string
	IP          []string
	Description string
}

type VMProvider interface {
	// GetDatacenters returns a list of all datacenters managed by this provider
	GetDatacenters() ([]Datacenter, error)
	// GetDcClusters gets a list of clusters for the given datacenter ID
	GetDcClusters(datacenterID string) ([]Cluster, error)
	// GetClusterVMs returns a list of VMs for the given cluster ID
	GetClusterVMs(clusterID string) ([]VM, error)
	// GetName returns the name of the VMProvider (eg. vmware)
	GetName() string
}

var ErrNotImplemented = errors.New("method has not been implemented")
