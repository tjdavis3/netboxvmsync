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
	client             *nb.Client
	filter             *string
	log                pkg.Logger
	derivedDataCenters []sync.Datacenter
	derivedClusters    []DerivedCluster
}

func NewNetboxProvider(netboxClient *nb.Client, filter *string, logger pkg.Logger) (*Netbox, error) {
	nb := &Netbox{client: netboxClient, filter: filter}
	if log, ok := logger.(*slog.Logger); ok {
		nb.log = log.With("provider", nb.GetName())
	}
	err := nb.deriveGroupsAndClusters()
	return nb, err
}

// GetDatacenters returns a list of all datacenters managed by this provider
func (nb *Netbox) GetDatacenters() ([]sync.Datacenter, error) {
	var datacenters []sync.Datacenter
	var ids []string
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
		ids = append(ids, dc.ID)
		datacenters = append(datacenters, dc)
	}
	for _, derived := range nb.derivedDataCenters {
		if !objInArray(derived, ids) {
			datacenters = append(datacenters, derived)
		}
	}
	return datacenters, nil
}

func objInArray(obj sync.ObjectID, objArray []string) bool {
	found := false
	for _, arrObj := range objArray {
		if obj.GetID() == arrObj {
			found = true
			break
		}
	}
	return found
}

// GetDcClusters gets a list of clusters for the given datacenter ID
func (nb *Netbox) GetDcClusters(datacenterID string) ([]sync.Cluster, error) {
	var clusters []sync.Cluster
	var ids []string
	var args string
	if nb.filter != nil {
		args = *nb.filter
	}
	filter := fmt.Sprintf("group_id=%s&%s", datacenterID, args)
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
		ids = append(ids, cluster.ID)
		clusters = append(clusters, cluster)
	}
	for _, cobj := range nb.derivedClusters {
		if cobj.GroupID == datacenterID {
			if !objInArray(cobj, ids) {
				clusters = append(clusters, cobj.Cluster)
			}
		}
	}
	return clusters, nil
}

// deriveGroupsAndClusters finds all filtered VMs and determines their clusters and groups
func (nb *Netbox) deriveGroupsAndClusters() error {
	result := &VMSearchResponse{}
	var args string

	groups := make(map[int]netbox.DisplayIDName)

	clusterIDX := make(map[int64]EmbeddedCluster)
	if nb.filter != nil {
		args = *nb.filter
	}
	err := nb.client.Search("virtualmachine", result, args)
	if err != nil {
		nb.log.Error("could not find virtual machines", "error", err)
		return err
	}
	for _, vm := range result.Results {
		clusterIDX[vm.Cluster.ID] = vm.Cluster
	}
	for result.Next != nil {
		_, err := nb.client.GetByURL(*result.Next, result)
		if err != nil {
			nb.log.Error("error getting more vms", "error", err)
			return err
		}
		for _, vm := range result.Results {
			clusterIDX[vm.Cluster.ID] = vm.Cluster
		}
	}
	nb.derivedClusters = make([]DerivedCluster, 0)
	for _, clstr := range clusterIDX {
		cluster := &netbox.Cluster{}
		_, err = nb.client.GetByURL(clstr.URL, cluster)
		groups[cluster.Group.ID] = cluster.Group
		nb.derivedClusters = append(nb.derivedClusters,
			DerivedCluster{
				Cluster: sync.Cluster{
					ID:          fmt.Sprint(cluster.ID),
					Name:        cluster.Name,
					Description: cluster.Description,
				},
				GroupID: fmt.Sprint(cluster.Group.ID),
			},
		)
	}
	nb.derivedDataCenters = make([]sync.Datacenter, 0)
	for _, grp := range groups {
		nb.derivedDataCenters = append(nb.derivedDataCenters,
			sync.Datacenter{
				ID:          fmt.Sprint(grp.ID),
				Name:        grp.Name,
				Description: grp.Display,
			},
		)
	}
	return nil
}

// GetClusterVMs returns a list of VMs for the given cluster ID
func (nb *Netbox) GetClusterVMs(clusterID string) ([]sync.VM, error) {
	var vms []sync.VM
	var searchArgs string
	if nb.filter != nil {
		searchArgs = *nb.filter
	}

	clusterResp := &netbox.ClusterResponse{}
	err := nb.client.Search("cluster", clusterResp, fmt.Sprintf("id=%s", clusterID), searchArgs)
	if err != nil {
		nb.log.Error("error searching for cluster", "error", err)
		return nil, err
	}
	if clusterResp.Count != 0 {
		searchArgs = ""
	}

	nbvms, err := nb.client.SearchVMs(fmt.Sprintf("cluster_id=%s", clusterID), searchArgs)
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
