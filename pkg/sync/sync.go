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
					s.log.Info("adding new VM", "cluster", cluster.Name, "VM", vm.Name)
					newvm := &netbox.NewVM{}
					newvm.Name = vm.Name
					newvm.ClusterID = nbCluster.ID
					newvm.Diskspace = vm.Diskspace
					newvm.Memory = vm.Memory

					nbVm, err = s.netbox.AddVM(*newvm)
					if err != nil {
						s.log.Error("failed to add vm", "VM", vm.Name)
					}
					payload := make(map[string]interface{})
					cf := make(map[string]interface{})
					cf["vmid"] = vm.ID
					payload["custom_fields"] = cf
					if err = s.netbox.UpdateObjectByURL(nbVm.URL, payload); err != nil {
						s.log.Error("could not set vmID on vm", "vm", vm.Name, "error", err)
					}
				}
				// Update VM if changed
				_ = nbVm
			}
		}
	}
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
