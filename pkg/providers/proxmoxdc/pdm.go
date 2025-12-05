package proxmoxdc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ringsq/netboxvmsync/pkg"
	"github.com/ringsq/netboxvmsync/pkg/sync"
	pdm "github.com/srerun/go-proxmox-pdm"
)

var _ sync.VMProvider = (*PDMProvider)(nil)

const gb = 1073741824
const mb = 1048576

type PDMProvider struct {
	client    *pdm.Client
	log       pkg.Logger
	resources pdm.Resources
}

var ErrNotImplemented = errors.New("not implemented")

// NewVmwareProvider creates a new VM sync provider using vmware vcenter
func NewProxmoxDCProvider(baseURL string, username string, password string, logger pkg.Logger) (*PDMProvider, error) {
	insecureHTTPClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	pdmprov := &PDMProvider{log: logger}
	if log, ok := logger.(*slog.Logger); ok {
		pdmprov.log = log.With("provider", pdmprov.GetName())
	}
	pdmprov.log.Info("Connecting...", "user", username)
	pdmprov.client = pdm.NewClient(fmt.Sprintf("%s/api2/json", baseURL),
		pdm.WithHTTPClient(&insecureHTTPClient),
		pdm.WithAPIToken(username, password),
	)
	version, err := pdmprov.client.Version(context.Background())
	if err != nil {
		return pdmprov, err
	}
	pdmprov.log.Info("connected to proxmox datacenter manager", "version", version)
	return pdmprov, nil
}

func (p *PDMProvider) GetName() string {
	return "Proxmox"
}

// GetDatacenters returns a list of all datacenters managed by this provider
func (p *PDMProvider) GetDatacenters() ([]sync.Datacenter, error) {
	var err error
	dc := sync.Datacenter{}
	dc.Name = p.GetName()
	dc.ID = p.GetName()
	dc.Description = "Proxmox Clusters"
	p.resources, err = p.client.Resources(context.Background())
	return []sync.Datacenter{dc}, err
}

// GetDcClusters gets a list of clusters for the given datacenter ID
func (p *PDMProvider) GetDcClusters(datacenterID string) ([]sync.Cluster, error) {
	clusters := make([]sync.Cluster, 0)
	for _, cluster := range p.resources {
		if cluster.Remote == "Backup" { // skip backup server entry
			continue
		}
		clusters = append(clusters, sync.Cluster{Name: cluster.Remote, ID: cluster.Remote})
	}
	return clusters, nil
}

// GetClusterVMs returns a list of VMs for the given cluster ID
func (p *PDMProvider) GetClusterVMs(clusterID string) ([]sync.VM, error) {
	vms := make([]sync.VM, 0)
	for _, cluster := range p.resources {
		if cluster.Remote != clusterID {
			continue
		}
		clustervms := pdm.FilterClusterResourcesByType(cluster.Resources, pdm.VMType)
		for _, resource := range clustervms {
			if resource.Template { // skip templates
				continue
			}
			vm := sync.VM{}
			id := strings.Split(resource.ID, "/")
			vm.ID = fmt.Sprint(id[len(id)-1])
			vm.Name = resource.Name
			vm.Description = resource.Type
			vm.Memory = int(resource.MaxMem / mb)
			vm.Diskspace = int(resource.MaxDisk / gb)
			vm.VCPUs = float32(resource.MaxCPU)
			if resource.Status == "running" {
				vm.Status = "active"
			} else {
				vm.Status = "offline"
			}

			vm.Network = make([]sync.NIC, 0)

			vmid, err := strconv.Atoi(vm.ID)
			if err == nil {
				cfg, err := p.client.GetVMConfig(context.Background(), clusterID, vmid)
				if err == nil {
					for key, value := range cfg {
						if key == "description" {
							vm.Description = fmt.Sprint(value)
						}
						if strings.HasPrefix(key, "net") {
							nicData := splitFieldValue(fmt.Sprint(value))
							nic := sync.NIC{}
							nic.MAC = nicData["mac"]
							nic.Description = nicData["description"]
							nic.ID = key
							nic.Name = key
							netdev := strings.Split(key, "net")
							if len(netdev) == 2 {
								nicID := netdev[1]
								ipconfig := fmt.Sprintf("ipconfig%s", nicID)
								if ipdata, ok := cfg[ipconfig]; ok {
									ipfields := splitFieldValue(fmt.Sprint(ipdata))
									if ip, ok := ipfields["ip"]; ok {
										nic.IP = []string{ip}
									}
								}
							}
							vm.Network = append(vm.Network, nic)
						}
					}
				}
			}

			vms = append(vms, vm)
		}
	}
	return vms, nil
}

// splitFieldValues takes the field of the form key=value,key=value
// and returns it as a map of [key]value
func splitFieldValue(field string) map[string]string {
	data := make(map[string]string)
	fields := strings.Split(field, ",")
	idx := 0
	for _, f := range fields {
		values := strings.Split(f, "=")
		data[values[0]] = values[1]
		if len(values) == 2 {
			if idx == 0 {
				data["description"] = field
				data["mac"] = values[1]
				data["driver"] = values[0]
			}
		}
		idx++
	}
	return data
}
