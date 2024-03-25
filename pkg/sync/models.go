package sync

import (
	"errors"

	"github.com/rsapc/netbox"
)

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

type NBVM struct {
	netbox.DeviceOrVM
	Interfaces []netbox.Interface
	IPs        []NetboxIP
}

type Netbox interface {
	GetVM(id string) (vm NBVM, err error)
	Compare(vm NBVM, pVm VM) map[string]interface{}
	UpdateVM(map[string]interface{})
	AddVM(vm VM)
	DeleteVM(url string)
}

var ErrNotImplemented = errors.New("method has not been implemented")

type CustomField struct {
	Name     string
	Label    string
	Readonly bool
	Types    []string
}

type NetboxIP struct {
	ID           int
	URL          string
	Address      string
	Status       string
	InterfaceID  int
	Description  *string
	CustomFields *map[string]any
}
