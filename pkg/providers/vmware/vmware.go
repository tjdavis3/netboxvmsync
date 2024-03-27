package vmware

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ringsq/netboxvmsync/pkg"
	"github.com/ringsq/netboxvmsync/pkg/sync"
	"github.com/ringsq/vcenterapi/pkg/vcenter"
)

const VM_STATUS_ON = "POWERED_ON"

type VmwareProvider struct {
	vcenter *vcenter.Vcenter
	log     pkg.Logger
}

// NewVmwareProvider creates a new VM sync provider using vmware vcenter
func NewVmwareProvider(baseURL string, username string, password string, logger pkg.Logger) (*VmwareProvider, error) {
	vmw := &VmwareProvider{log: logger}
	if log, ok := logger.(*slog.Logger); ok {
		vmw.log = log.With("provider", vmw.GetName())
	}
	vmw.log.Info("Connecting...", "user", username)
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
		vmDetail.VCPUs = float32(listVM.CPUCount)
		vmDetail.Memory = listVM.MemorySizeMiB
		if listVM.PowerState == VM_STATUS_ON {
			vmDetail.Status = "active"
		} else {
			vmDetail.Status = "offline"
		}
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
			adapter.MAC = strings.ToUpper(nic.MacAddress)
			adapter.Name = nic.Label
			adapter.Description = fmt.Sprintf("%s %s to %s/%s", nic.Type, nic.State, nic.Backing.Type, nic.Backing.Network)
			for _, intf := range vm.VMinterfaces {
				if intf.MacAddress == nic.MacAddress {
					for _, ip := range intf.IP.IPAddresses {
						adapter.IP = append(adapter.IP, fmt.Sprintf("%s/%d", ip.IPAddress, ip.PrefixLength))
					}
				}
			}
			vmDetail.Network = append(vmDetail.Network, adapter)
		}
		vms = append(vms, vmDetail)
	}
	return vms, nil
}
