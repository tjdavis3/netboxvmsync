package sync

import (
	"fmt"
	"net/url"

	"github.com/rsapc/netbox"
)

func (s *Sync) GetVMbyName(clusterID int, name string) (NBVM, error) {
	vm := &NBVM{}
	args := []string{
		fmt.Sprintf("name=%s", url.QueryEscape(name)),
		fmt.Sprintf("cluster_id=%d", clusterID),
	}
	nbVms, err := s.netbox.SearchVMs(args...)
	if err != nil {
		return *vm, err
	}
	if len(nbVms) == 0 {
		return *vm, netbox.ErrNotFound
	}
	vm = &NBVM{nbVms[0], nil, nil}
	err = s.loadVMinterfacesAndIP(vm)
	return *vm, err
}

func (s *Sync) GetVM(id string) (NBVM, error) {
	vm := &NBVM{}
	nbVms, err := s.netbox.SearchVMs(fmt.Sprintf("cf_vmid=%s", id))
	if err != nil {
		return *vm, err
	}
	if len(nbVms) == 0 {
		return *vm, netbox.ErrNotFound
	}
	vm = &NBVM{nbVms[0], nil, nil}
	err = s.loadVMinterfacesAndIP(vm)
	return *vm, err
}

func (s *Sync) loadVMinterfacesAndIP(vm *NBVM) error {
	// Get interfaces
	intfs, err := s.netbox.GetInterfacesForObject("virtualmachine", int64(vm.ID))
	if err != nil {
		s.log.Error("could not load interfaces", "vm", vm.Name, "error", err)
		return err
	}
	vm.Interfaces = intfs

	// GetIPs
	ips := make([]NetboxIP, 0)
	ipSearchResult := &netbox.IPSearchResults{}
	err = s.netbox.Search("ipaddress", ipSearchResult, fmt.Sprintf("virtual_machine_id=%d", vm.ID))
	if err != nil {
		return err
	}
	ips = s.updateIPs(ips, ipSearchResult)
	for ipSearchResult.Next != nil {
		_, err = s.netbox.GetByURL(fmt.Sprint(ipSearchResult.Next), ipSearchResult)
		if err != nil {
			return err
		}
		ips = s.updateIPs(ips, ipSearchResult)
	}
	vm.IPs = ips
	return nil
}

func (s *Sync) updateIPs(ips []NetboxIP, results *netbox.IPSearchResults) []NetboxIP {
	for _, ip := range results.Results {
		nbip := NetboxIP{
			ID:          ip.ID,
			Address:     ip.Address,
			URL:         ip.URL,
			Status:      ip.Status.Value,
			InterfaceID: ip.AssignedObjectID,
			Description: &ip.Description,
		}

		ips = append(ips, nbip)
	}
	return ips
}
