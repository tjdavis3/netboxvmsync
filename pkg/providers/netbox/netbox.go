package netbox

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ringsq/netboxvmsync/pkg"
	"github.com/ringsq/netboxvmsync/pkg/sync"
	"github.com/rsapc/netbox"
	nb "github.com/rsapc/netbox"
)

type Netbox struct {
	client *nb.Client
	filter *string
	log    pkg.Logger
}

func NewNetboxProvider(netboxClient *nb.Client, filter *string, logger pkg.Logger) (*Netbox, error) {
	nb := &Netbox{client: netboxClient, filter: filter}
	if log, ok := logger.(*slog.Logger); ok {
		nb.log = log.With("provider", nb.GetName())
	}
	return nb, nil
}

// GetDatacenters returns a list of all datacenters managed by this provider
func (nb *Netbox) GetDatacenters() ([]sync.Datacenter, error) {
	var datacenters []sync.Datacenter
	groups, err := nb.client.GetClusterGroups(nb.filter)
	if err != nil {
		return datacenters, err
	}
	for _, group := range groups {
		dc := sync.Datacenter{
			ID:          fmt.Sprint(group.ID),
			Name:        group.Name,
			Description: group.Description,
		}
		datacenters = append(datacenters, dc)
	}
	return datacenters, nil
}

// GetDcClusters gets a list of clusters for the given datacenter ID
func (nb *Netbox) GetDcClusters(datacenterID string) ([]sync.Cluster, error) {
	var clusters []sync.Cluster

	filter := fmt.Sprintf("group_id=%s&%v", datacenterID, nb.filter)
	nbClusters, err := nb.client.GetClusters(&filter)
	if err != nil {
		return clusters, err
	}
	for _, nbCluster := range nbClusters {
		cluster := sync.Cluster{
			ID:          fmt.Sprint(nbCluster.ID),
			Name:        nbCluster.Name,
			Description: nbCluster.Description,
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

// GetClusterVMs returns a list of VMs for the given cluster ID
func (nb *Netbox) GetClusterVMs(clusterID string) ([]sync.VM, error) {
	var vms []sync.VM

	nbvms, err := nb.client.SearchVMs(fmt.Sprintf("cluster_id=%s", clusterID))
	if err != nil {
		nb.log.Error("error getting VMs", "cluster", clusterID, "error", err)
		return vms, err
	}
	for _, nbvm := range nbvms {
		vm := &sync.VM{}
		vm.ID = fmt.Sprintf("%d", nbvm.ID)
		vm.Description = nbvm.Description
		vm.Diskspace = nbvm.Diskspace
		vm.Memory = nbvm.Memory
		vm.Status = nbvm.Status.Value
		vm.Name = nbvm.Name
		vm.Network = make([]sync.NIC, 0)
		vm.VCPUs = nbvm.VCPUs
		nb.loadVMinterfacesAndIP(vm)
		vms = append(vms, *vm)
	}
	return vms, nil
}

// GetName returns the name of the VMProvider (eg. vmware)
func (nb *Netbox) GetName() string {
	return "Netbox"
}

func (nb *Netbox) loadVMinterfacesAndIP(vm *sync.VM) error {
	// Get interfaces
	id, err := strconv.ParseInt(vm.ID, 0, 64)
	if err != nil {
		nb.log.Warn("could not convert the VM id to int64", "id", vm.ID, "error", err)
		return err
	}
	intfs, err := nb.client.GetInterfacesForObject("virtualmachine", id)
	if err != nil {
		nb.log.Error("could not load interfaces", "vm", vm.Name, "error", err)
		return err
	}
	if vm.Network == nil {
		vm.Network = make([]sync.NIC, 0)
	}
	for _, intf := range intfs {
		var mac string
		if intf.MacAddress != nil {
			mac = *intf.MacAddress
		}
		net := sync.NIC{
			ID:          fmt.Sprint(intf.ID),
			Name:        intf.Name,
			MAC:         mac,
			IP:          make([]string, 0),
			Description: intf.Description,
		}
		vm.Network = append(vm.Network, net)
	}

	// GetIPs
	ipSearchResult := &netbox.IPSearchResults{}
	err = nb.client.Search("ipaddress", ipSearchResult, fmt.Sprintf("virtual_machine_id=%v", vm.ID))
	if err != nil {
		return err
	}
	for _, ip := range ipSearchResult.Results {
		vm.Network = updateIP(vm, ip.Address, fmt.Sprint(ip.AssignedObjectID))
	}
	for ipSearchResult.Next != nil {
		_, err = nb.client.GetByURL(fmt.Sprint(ipSearchResult.Next), ipSearchResult)
		if err != nil {
			return err
		}
		for _, ip := range ipSearchResult.Results {
			vm.Network = updateIP(vm, ip.Address, fmt.Sprint(ip.AssignedObjectID))
		}
	}
	return nil
}

func updateIP(vm *sync.VM, ip string, intfID string) []sync.NIC {
	var ips []string
	var nics []sync.NIC
	for _, intf := range vm.Network {
		if intf.ID == intfID {
			ips = intf.IP
			ips = append(ips, ip)
			intf.IP = ips
		}
		nics = append(nics, intf)
	}
	return nics
}
