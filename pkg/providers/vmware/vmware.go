package vmware

import (
	"log/slog"

	"github.com/ringsq/netboxvcenter/pkg/sync"
	"github.com/ringsq/vcenterapi/pkg/vcenter"
	"github.com/rsapc/hookcmd/models"
)

type VmwareProvider struct {
	vcenter *vcenter.Vcenter
	log     models.Logger
}

// NewVmwareProvider creates a new VM sync provider using vmware vcenter
func NewVmwareProvider(baseURL string, username string, password string, logger models.Logger) (*VmwareProvider, error) {
	vmw := &VmwareProvider{log: logger}
	if log, ok := logger.(*slog.Logger); ok {
		vmw.log = log.With("provider", vmw.GetName())
	}
	vcntr, err := vcenter.NewClient(baseURL, username, password, vmw.log)
	if err != nil {
		vmw.log.Error("failed to connect to vmware", "error", err)
		return nil, err
	}
	vmw.vcenter = vcntr

	return vmw, nil
}

func (v *VmwareProvider) GetName() string {
	return "VMWare"
}

func (v *VmwareProvider) GetDatacenters() ([]sync.Datacenter, error) {
	sDcs := make([]sync.Datacenter, 0)
	dcs, err := v.vcenter.ListDatacenters()
	if err != nil {
		v.log.Error("could not list datacenters", "error", err)
		return sDcs, err
	}

	for _, dc := range dcs {
		sDc := sync.Datacenter{
			ID:   dc.ID,
			Name: dc.Name,
		}
		sDcs = append(sDcs, sDc)
	}

	return sDcs, nil
}
func (v *VmwareProvider) GetDcClusters(datacenterID string) ([]sync.Cluster, error) {
	clusters := make([]sync.Cluster, 0)
	vcs, err := v.vcenter.ListDcClusters(datacenterID)
	if err != nil {
		v.log.Error("could not retrieve clusters", "error", err)
		return clusters, err
	}
	for _, vc := range vcs {
		sVc := sync.Cluster{
			ID:   vc.ID,
			Name: vc.Name,
		}
		clusters = append(clusters, sVc)
	}
	return clusters, nil
}

func (v *VmwareProvider) GetClusterVMs(clusterID string) ([]sync.VM, error) {
	vms := make([]sync.VM, 0)
	vcVMs, err := v.vcenter.ListClusterVMs(clusterID)
	if err != nil {
		v.log.Error("could not list VMs", "error", err)
		return vms, err
	}
	for _, listVM := range vcVMs {
		vmDetail := sync.VM{}
		vm, err := v.vcenter.GetVM(listVM.ID)
		if err != nil {
			v.log.Error("failed to get VM details", "error", err)
		}
		vmDetail.ID = listVM.ID
		vmDetail.Name = listVM.Name
		vmDetail.Memory = listVM.MemorySizeMiB
		for _, disk := range vm.Disks {
			vmDetail.Diskspace = vmDetail.Diskspace + disk.Capacity
		}
		if vmDetail.Diskspace > 0 {
			vmDetail.Diskspace = vmDetail.Diskspace / 1000000000
		}
		vmDetail.Network = make([]sync.NIC, 0)
		for nicID, nic := range vm.Nics {
			adapter := sync.NIC{}
			adapter.ID = nicID
			adapter.MAC = nic.MacAddress
			adapter.Name = nic.Label
			vmDetail.Network = append(vmDetail.Network, adapter)
		}
		vms = append(vms, vmDetail)
	}
	return vms, nil
}
