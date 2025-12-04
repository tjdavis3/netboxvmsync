package sync

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

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
				s.processVM(nbCluster, vm)
			}
			_ = s.Prune(nbCluster, vms)
		}
	}
}

func (s *Sync) processVM(nbCluster netbox.Cluster, vm VM) {
	found := false
	nbVM, err := s.GetVM(nbCluster.ID, vm.ID)
	if err != nil {
		if errors.Is(err, netbox.ErrNotFound) {
			nbVM, err = s.GetVMbyName(nbCluster.ID, vm.Name)
			if err == nil {
				found = true
				s.setIDandProvider(nbVM.URL, vm.ID)
			} else if errors.Is(err, netbox.ErrNotFound) {
				if err = s.AddVMtoCluster(nbCluster.ID, vm); err != nil {
					s.log.Error("error adding VM", "error", err)
				}
			}
		}
	} else {
		found = true
	}
	if found {
		// Update VM if changed
		s.UpdateVM(nbVM, vm)
	}

}

// UpdateVM compares the netbox VM to the provider VM and makes updates as necessary
func (s *Sync) UpdateVM(nbVM NBVM, vm VM) error {
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
	if nbVM.VCPUs != vm.VCPUs {
		editVM.VCPUs = vm.VCPUs
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
	for _, intf := range vm.Network {
		found, nbint := findInterface(intf, nbVM.Interfaces)
		if found {
			s.updateVMInterface(nbint, intf)
			s.updateInterfaceIPs(nbVM, nbint, intf)
		} else {
			s.addInterface(nbVM.ID, intf)
		}
	}

	return nil
}

func (s *Sync) updateInterfaceIPs(nbVM NBVM, nbint netbox.Interface, intf NIC) {
	nbIPs := getInterfaceIPs(nbVM, nbint.ID)
	for _, ip := range intf.IP {
		found := false
		for _, nip := range nbIPs {
			if nip.Address == ip {
				found = true
			}
		}
		if !found {
			s.addInterfaceIP(nbint.ID, ip, intf.ID)
		}
	}
}

func getInterfaceIPs(nbvm NBVM, intID int) []NetboxIP {
	ips := []NetboxIP{}
	for _, ip := range nbvm.IPs {
		if ip.InterfaceID == intID {
			ips = append(ips, ip)
		}
	}
	return ips
}

func (s *Sync) updateVMInterface(nbint netbox.Interface, nic NIC) error {
	data := make(map[string]interface{})
	if nbint.MacAddress != nil && nic.MAC != "" {
		if !strings.EqualFold(*nbint.MacAddress, nic.MAC) {
			data["mac_address"] = nic.MAC
		}
	}
	if len(data) > 0 {
		return s.netbox.UpdateObjectByURL(nbint.URL, data)
	}
	return nil
}

// interfaceExists looks through the Netbox interfaces to see if
// one exists with the vmid that matches the interface ID
func findInterface(intf NIC, nbInts []netbox.Interface) (bool, netbox.Interface) {
	for _, nbint := range nbInts {
		if vmid, ok := nbint.CustomFields["vmid"]; ok {
			if fmt.Sprint(vmid) == intf.ID {
				return true, nbint
			}
		}
	}
	return false, netbox.Interface{}
}

// AddVMtoCluster creates a new VM under the given cluster ID
func (s *Sync) AddVMtoCluster(clusterID int, vm VM) error {
	s.log.Info("adding new VM", "cluster", clusterID, "VM", vm.Name)
	newvm := &netbox.NewVM{}
	newvm.Name = vm.Name
	newvm.ClusterID = clusterID
	newvm.Diskspace = vm.Diskspace
	newvm.Memory = vm.Memory
	newvm.VCPUs = vm.VCPUs
	newvm.Status = vm.Status

	nbVm, err := s.netbox.AddVM(*newvm)
	if err != nil {
		s.log.Error("failed to add vm", "VM", vm.Name)
		return err
	}

	// Add the vm id to the vmid custom field value
	if err = s.setIDandProvider(nbVm.URL, vm.ID); err != nil {
		s.log.Error("could not set vmID on vm", "vm", vm.Name, "error", err)
	}

	// Add the interfaces
	for _, nic := range vm.Network {
		s.addInterface(nbVm.ID, nic)
	}

	return nil
}

func (s *Sync) addInterface(vmid int, nic NIC) {
	intf := netbox.InterfaceEdit{
		Name:        &nic.Name,
		VM:          &vmid,
		Description: nic.Description,
	}
	if nic.MAC != "" {
		intf.MacAddress = &nic.MAC
	}
	newIntf, err := s.netbox.AddInterface("virtualmachine", int64(vmid), intf)
	if err != nil {
		s.log.Error("could not add interface", "vm", vmid, "nic", nic.Name, "error", err)
	} else {
		s.setIDandProvider(newIntf.URL, nic.ID)
		for _, ipaddr := range nic.IP {
			s.addInterfaceIP(newIntf.ID, ipaddr, nic.ID)
		}
	}
}

func (s *Sync) addInterfaceIP(intfID int, ipaddr string, nicID string) {
	if ip, err := s.netbox.AddIP(ipaddr); err == nil { // Should we check if it exists first?
		ipdata := make(map[string]interface{})
		ipdata["assigned_object_type"] = "virtualization.vminterface"
		ipdata["assigned_object_id"] = intfID
		ipdata["custom_fields"] = s.buildIDandProviderFields(nicID)
		if err = s.netbox.UpdateObject("ip-address", int64(ip.ID), ipdata); err != nil {
			s.log.Error("Could not assign ipaddress", "IP", ip.Address, "device", intfID, "error", err)
		}
	}

}

// VerifyCustomFields ensures required fields exist in Netbox
func (s *Sync) VerifyCustomFields() error {
	var err error
	fields := []CustomField{
		{
			Name:     "vmid",
			Label:    "Provider VM ID",
			Readonly: true,
			Types:    []string{"virtualmachine", "ipaddress", "cluster", "cluster-group", "vminterface"},
		},
		{
			Name:     "vmprovider",
			Label:    "Virtualization Provider",
			Readonly: true,
			Types:    []string{"virtualmachine", "ipaddress", "cluster", "cluster-group", "vminterface"},
		},
	}
	for _, field := range fields {
		ferr := s.VerifyCustomField(field)
		if ferr != nil && err == nil {
			err = ferr
		}
	}
	return err
}

func (s *Sync) VerifyCustomField(field CustomField) error {
	exist, err := s.netbox.CustomFieldExists(field.Name)
	if err != nil {
		return err
	}
	if !exist {
		return s.netbox.AddCustomField(field.Name, field.Label, field.Readonly, field.Types...)
	}
	return nil
}

// Verify ClusterType exists
func (s *Sync) VerifyClusterType() error {
	_, err := s.netbox.GetClusterType(s.vmProvider.GetName())
	if err != nil {
		if errors.Is(err, netbox.ErrNotFound) {
			cluster, err := s.netbox.AddClusterType(s.vmProvider.GetName())
			if err != nil {
				s.log.Error("could not create cluster type", "type", s.vmProvider.GetName(), "error", err)
				return err
			}
			return s.setCustomFields(cluster.URL, map[string]any{"vmprovider": s.vmProvider.GetName()})
		} else {
			return err
		}
	}
	return nil
}

func (s *Sync) setCustomFields(url string, fields map[string]any) error {
	data := make(map[string]interface{})
	data["custom_fields"] = fields

	return s.netbox.UpdateObjectByURL(url, data)
}

func (s *Sync) setIDandProvider(url string, vmid string) error {
	cf := s.buildIDandProviderFields(vmid)
	return s.setCustomFields(url, cf)
}

func (s *Sync) buildIDandProviderFields(vmid string) map[string]any {
	cf := make(map[string]interface{})
	cf["vmid"] = vmid
	cf["vmprovider"] = s.vmProvider.GetName()
	return cf
}

// Prune will look through all VMs in Netbox for the given cluster
// that were created by the sync
// if they are "active" in Netbox but do not exist in VMWARE their status
// will be set to Decommissioning.  If it's already decommissioning the
// device will be deleted
func (s *Sync) Prune(cluster netbox.Cluster, pvms []VM) error {
	s.log.Info("Pruning removed VMs", "cluster", cluster.Name)
	vms, err := s.netbox.SearchVMs(fmt.Sprintf("cf_vmprovider=%s", url.QueryEscape(s.vmProvider.GetName())), fmt.Sprintf("cluster_id=%d", cluster.ID))
	if err != nil {
		s.log.Error("error retrieving netbox VMs for cluster", "cluster", cluster.Name, "error", err)
		return err
	}
	for _, vm := range vms {
		vmid, ok := vm.CustomFieldsMap["vmid"]
		if !ok || vmid == nil {
			continue
		}
		if err = s.validateNBvm(vm, pvms); err != nil {
			s.log.Error("Prune error", "error", err)
		}
	}
	return err
}

func (s *Sync) validateNBvm(vm netbox.DeviceOrVM, pvms []VM) error {
	var err error
	found := false
	vmid, ok := vm.CustomFieldsMap["vmid"]
	if !ok {
		return nil
	}
	for _, pvm := range pvms {
		if fmt.Sprint(vmid) == pvm.ID {
			found = true
			break
		}
	}
	if !found {
		data := make(map[string]interface{})
		if vm.Status.Value == "active" || vm.Status.Value == "offline" {
			data["status"] = "decommissioning"
			s.log.Info("decommissioning VM", "vm", vm.Name)
			err = s.netbox.UpdateObjectByURL(vm.URL, data)
		} else if vm.Status.Value == "decommissioning" {
			now := time.Now()
			updated, err := time.Parse(time.RFC3339, vm.LastUpdated)
			if err != nil {
				s.log.Error("Invalid last updated time", "error", err)
				return err
			}
			removeOn := updated.Add(30 * 24 * time.Hour)
			if now.After(removeOn) {
				s.log.Warn("Deleting VM", "vm", vm.Name)
				err = s.netbox.DeleteObjectByURL(vm.URL)
			}
		}
	}
	return err
}
