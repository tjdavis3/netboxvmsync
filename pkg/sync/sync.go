package sync

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/ringsq/netboxvmsync/pkg"
	"github.com/rsapc/netbox"
)

type Sync struct {
	netbox     *netbox.Client
	vmProvider VMProvider
	log        pkg.Logger
}

func NewSyncService(netbox *netbox.Client, provider VMProvider, logger pkg.Logger) *Sync {
	sync := &Sync{netbox: netbox, vmProvider: provider, log: logger}
	if log, ok := logger.(*slog.Logger); ok {
		sync.log = log.With("service", "netboxvcenter sync")
	}

	return sync
}

func (s *Sync) StartSync() {
	if err := s.VerifyCustomFields(); err != nil {
		s.log.Error("could not verify or create custom fields", "error", err)
		os.Exit(1)
	}
	if err := s.VerifyClusterType(); err != nil {
		os.Exit(1)
	}
	s.log.Info("retrieving datacenters")
	dcs, _ := s.vmProvider.GetDatacenters()

	for _, dc := range dcs {
		s.log.Info("checking Netbox", "datacenter", dc.Name)
		nbGroup, err := s.netbox.GetOrAddClusterGroup(dc.Name)
		_ = nbGroup
		if err != nil {
			s.log.Error("could not get cluster group for datacenter", "error", err)
			os.Exit(1)
		}
		s.log.Info("getting clusters", "datacenter", dc.Name)
		clusters, err := s.vmProvider.GetDcClusters(dc.ID)
		if err != nil {
			log.Fatal(err)
		}
		for _, cluster := range clusters {
			nbCluster, err := s.netbox.GetOrAddCluster(dc.Name, cluster.Name, s.vmProvider.GetName())
			if err != nil {
				log.Fatal(err)
			}
			vms, err := s.vmProvider.GetClusterVMs(cluster.ID)
			if err != nil {
				log.Fatal(err)
			}
			for _, vm := range vms {
				var nbVm netbox.DeviceOrVM
				nbVms, err := s.netbox.SearchVMs(fmt.Sprintf("cf_vmid=%s", vm.ID))
				if (err != nil && errors.Is(err, netbox.ErrNotFound)) || len(nbVms) == 0 {
					if err = s.AddVMtoCluster(nbCluster.ID, vm); err != nil {
						s.log.Error("error adding VM", "error", err)
					}
				}
				// Update VM if changed
				s.UpdateVM(nbVm, vm)
			}
		}
	}
}

// UpdateVM compares the netbox VM to the provider VM and makes updates as necessary
func (s *Sync) UpdateVM(nbVM netbox.DeviceOrVM, vm VM) error {
	doUpdate := false
	editVM := &netbox.NewVM{Name: vm.Name}
	if nbVM.Diskspace != vm.Diskspace {
		editVM.Diskspace = vm.Diskspace
		doUpdate = true
	}
	if nbVM.Memory != vm.Memory {
		editVM.Memory = vm.Memory
		doUpdate = true
	}
	if nbVM.Status.Value != vm.Status {
		editVM.Status = vm.Status
		doUpdate = true
	}
	if doUpdate {
		if err := s.netbox.UpdateObject("virtualmachine", int64(nbVM.ID), editVM); err != nil {
			s.log.Error("could not update VM", "vm", nbVM.Name, "error", err)
			return err
		}
	}

	// Update any changed interfaces

	return nil
}

// AddVMtoCluster creates a new VM under the given cluster ID
func (s *Sync) AddVMtoCluster(clusterID int, vm VM) error {
	s.log.Info("adding new VM", "cluster", clusterID, "VM", vm.Name)
	newvm := &netbox.NewVM{}
	newvm.Name = vm.Name
	newvm.ClusterID = clusterID
	newvm.Diskspace = vm.Diskspace
	newvm.Memory = vm.Memory
	newvm.Status = vm.Status

	nbVm, err := s.netbox.AddVM(*newvm)
	if err != nil {
		s.log.Error("failed to add vm", "VM", vm.Name)
		return err
	}

	// Add the vm id to the vmid custom field value
	payload := make(map[string]interface{})
	cf := make(map[string]interface{})
	cf["vmid"] = vm.ID
	payload["custom_fields"] = cf
	if err = s.netbox.UpdateObjectByURL(nbVm.URL, payload); err != nil {
		s.log.Error("could not set vmID on vm", "vm", vm.Name, "error", err)
	}

	// Add the interfaces
	for _, nic := range vm.Network {
		intf := netbox.InterfaceEdit{
			Name:        &nic.Name,
			MacAddress:  &nic.MAC,
			VM:          &nbVm.ID,
			Description: nic.Description,
		}
		if err = s.netbox.AddInterface("virtualmachine", int64(nbVm.ID), intf); err != nil {
			s.log.Error("could not add interface", "vm", vm.Name, "nic", nic.Name, "error", err)
		}
	}

	return nil
}

// VerifyCustomFields ensures required fields exist in Netbox
func (s *Sync) VerifyCustomFields() error {
	exist, err := s.netbox.CustomFieldExists("vmid")
	if err != nil {
		return err
	}
	if !exist {
		return s.netbox.AddCustomField("vmid", "Provider VM ID", true, "virtualmachine")
	}
	return nil
}

// Verify ClusterType exists
func (s *Sync) VerifyClusterType() error {
	_, err := s.netbox.GetClusterType(s.vmProvider.GetName())
	if err != nil {
		if errors.Is(err, netbox.ErrNotFound) {
			_, err := s.netbox.AddClusterType(s.vmProvider.GetName())
			if err != nil {
				s.log.Error("could not create cluster type", "type", s.vmProvider.GetName(), "error", err)
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
