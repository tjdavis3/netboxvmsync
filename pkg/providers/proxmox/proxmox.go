package proxmox

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	proxapi "github.com/luthermonson/go-proxmox"
	"github.com/ringsq/netboxvmsync/pkg"
	"github.com/ringsq/netboxvmsync/pkg/sync"
)

var _ sync.VMProvider = (*ProxmoxProvider)(nil)

const gb = 1073741824
const mb = 1048576

type ProxmoxProvider struct {
	client *proxapi.Client
	log    pkg.Logger
}

var ErrNotImplemented = errors.New("not implemented")

// NewVmwareProvider creates a new VM sync provider using vmware vcenter
func NewProxmoxProvider(baseURL string, username string, password string, logger pkg.Logger) (*ProxmoxProvider, error) {
	insecureHTTPClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	prox := &ProxmoxProvider{log: logger}
	if log, ok := logger.(*slog.Logger); ok {
		prox.log = log.With("provider", prox.GetName())
	}
	prox.log.Info("Connecting...", "user", username)
	prox.client = proxapi.NewClient(fmt.Sprintf("%s/api2/json", baseURL),
		proxapi.WithHTTPClient(&insecureHTTPClient),
		proxapi.WithAPIToken(username, password),
	)
	version, err := prox.client.Version(context.Background())
	if err != nil {
		return prox, err
	}
	prox.log.Info("connected to proxmox", "version", version)
	return prox, nil
}

func (p *ProxmoxProvider) GetName() string {
	return "Proxmox"
}

// GetDatacenters returns a list of all datacenters managed by this provider
func (p *ProxmoxProvider) GetDatacenters() ([]sync.Datacenter, error) {
	dc := sync.Datacenter{}
	dc.Name = p.GetName()
	dc.ID = p.GetName()
	dc.Description = "Proxmox Clusters"
	return []sync.Datacenter{dc}, nil
}

// GetDcClusters gets a list of clusters for the given datacenter ID
func (p *ProxmoxProvider) GetDcClusters(datacenterID string) ([]sync.Cluster, error) {
	pCluster, err := p.client.Cluster(context.Background())
	if err != nil {
		return nil, err
	}
	cluster := sync.Cluster{Name: pCluster.Name, ID: pCluster.ID}
	return []sync.Cluster{cluster}, nil
}

// GetClusterVMs returns a list of VMs for the given cluster ID
func (p *ProxmoxProvider) GetClusterVMs(clusterID string) ([]sync.VM, error) {
	ctx := context.Background()
	cluster, err := p.client.Cluster(ctx)
	if err != nil {
		return nil, err
	}
	clusterRes, err := cluster.Resources(ctx, "vm")
	if err != nil {
		return nil, err
	}
	vms := make([]sync.VM, 0)
	for _, resource := range clusterRes {
		if resource.Template == 1 { // skip templates
			continue
		}
		vm := sync.VM{}
		vm.ID = fmt.Sprint(resource.VMID)
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
		node, err := p.client.Node(ctx, resource.Node)
		if err != nil {
			p.log.Warn("could not retrieve node for VM", "vm", vm.Name, "error", err)
			vms = append(vms, vm)
			continue
		}
		pVM, err := node.VirtualMachine(ctx, int(resource.VMID))
		if err != nil {
			p.log.Warn("could not retrieve VM details", "vm", vm.Name, "error", err)
			vms = append(vms, vm)
			continue
		}
		vm.Memory = int(pVM.VirtualMachineConfig.Memory)
		vm.Description = pVM.VirtualMachineConfig.Description
		vm.Network = make([]sync.NIC, 0)
		agentIFs, _ := pVM.AgentGetNetworkIFaces(ctx)
		nets := pVM.VirtualMachineConfig.MergeNets()
		for nicName, details := range nets {
			nicDetail := splitFieldValue(details)
			nic := sync.NIC{ID: nicName, Name: nicName}
			nic.MAC = nicDetail["virtio"]
			nic.Description = details
			if agentIF, found := findAgentIF(agentIFs, nic.MAC); found {
				nic.Name = agentIF.Name
				nic.IP = make([]string, 0)
				for _, pIP := range agentIF.IPAddresses {
					nic.IP = append(nic.IP, fmt.Sprintf("%s/%d", pIP.IPAddress, pIP.Prefix))
				}
			}
			vm.Network = append(vm.Network, nic)
		}
		vms = append(vms, vm)
	}
	return vms, nil
}

func findAgentIF(agentIFs []*proxapi.AgentNetworkIface, mac string) (*proxapi.AgentNetworkIface, bool) {
	for _, intf := range agentIFs {
		if strings.EqualFold(intf.HardwareAddress, mac) {
			return intf, true
		}
	}
	return nil, false
}

// splitFieldValues takes the field of the form key=value,key=value
// and returns it as a map of [key]value
func splitFieldValue(field string) map[string]string {
	data := make(map[string]string)
	fields := strings.Split(field, ",")
	for _, field := range fields {
		values := strings.Split(field, "=")
		if len(values) == 2 {
			data[values[0]] = values[1]
		}
	}
	return data
}
