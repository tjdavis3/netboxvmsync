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
	IP          string
	Description string
}

type VMProvider interface {
	GetDatacenters() ([]Datacenter, error)
	GetDcClusters(datacenterID string) ([]Cluster, error)
	GetClusterVMs(clusterID string) ([]VM, error)
	GetName() string
}

var ErrNotImplemented = errors.New("method has not been implemented")
