package main

import (
	"log"
	"log/slog"

	"github.com/ringsq/netboxvcenter/pkg/providers/vmware"
	"github.com/ringsq/netboxvcenter/pkg/sync"
	"github.com/rsapc/netbox"
)

type nbSite struct {
	ID  string
	URL string
}

var (
	netboxes map[string]nbSite
)

func init() {
	netboxes = make(map[string]nbSite)
	netboxes["tjd"] = nbSite{ID: "0123456789abcdef0123456789abcdef01234567",
		URL: "http://192.168.131.78:8000"}
	netboxes["ringsq"] = nbSite{
		ID:  "b973ce7c2989f230170a94376f47824c1215be57",
		URL: "https://netbox.ringsq.io"}
}

func main() {
	site := "tjd"
	url := netboxes[site].URL
	token := netboxes[site].ID
	nb := netbox.NewClient(url, token, slog.Default())

	provider, err := vmware.NewVmwareProvider("https://vcenter01.ringsquared.com", "todd.davis", "G4^$X5DxJp65kj2y@mSRHC2Kb", slog.Default())
	if err != nil {
		log.Fatal(err)
	}
	service := sync.NewSyncService(nb, provider, slog.Default())
	service.StartSync()
}
