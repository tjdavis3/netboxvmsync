package main

import (
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	nbProvider "github.com/ringsq/netboxvmsync/pkg/providers/netbox"
	"github.com/ringsq/netboxvmsync/pkg/providers/proxmox"
	"github.com/ringsq/netboxvmsync/pkg/providers/vmware"
	"github.com/ringsq/netboxvmsync/pkg/sync"
	"github.com/rsapc/netbox"
)

type nbSite struct {
	ID  string
	URL string
}

var (
	netboxes map[string]nbSite
)

type Config struct {
	NetboxURL      string  `env:"NETBOX_URL"`
	NetboxToken    string  `env:"NETBOX_TOKEN"`
	Provider       string  `env:"PROVIDER"`
	ProviderURL    string  `env:"PROVIDER_URL"`
	ProviderUser   string  `env:"PROVIDER_USER"`
	ProviderToken  string  `env:"PROVIDER_TOKEN"`
	ProviderFilter *string `env:"PROVIDER_FILTER"`
}

func main() {
	cfg := Configure(os.Getenv)
	nb := netbox.NewClient(cfg.NetboxURL, cfg.NetboxToken, slog.Default())
	slog.Info("Created Netbox client", "url", cfg.NetboxURL)
	var provider sync.VMProvider
	var err error

	switch strings.ToLower(cfg.Provider) {
	case "proxmox":
		provider, err = proxmox.NewProxmoxProvider(cfg.ProviderURL, cfg.ProviderUser, cfg.ProviderToken, slog.Default())
	case "netbox":
		nbProvClient := netbox.NewClient(cfg.ProviderURL, cfg.ProviderToken, slog.Default())
		provider, err = nbProvider.NewNetboxProvider(nbProvClient, cfg.ProviderFilter, slog.Default())
	default:
		provider, err = vmware.NewVmwareProvider(cfg.ProviderURL, cfg.ProviderUser, cfg.ProviderToken, slog.Default())
	}
	if err != nil {
		log.Fatal(err)
	}
	service := sync.NewSyncService(nb, provider, slog.Default())
	service.StartSync()
}

func Configure(getenv func(string) string) Config {
	cfg := Config{}
	godotenv.Overload()
	cfg.NetboxURL = getenv("NETBOX_URL")
	cfg.NetboxToken = getenv("NETBOX_TOKEN")
	cfg.ProviderURL = getenv("PROVIDER_URL")
	cfg.ProviderUser = getenv("PROVIDER_USER")
	cfg.ProviderToken = getenv("PROVIDER_TOKEN")
	cfg.Provider = getenv("PROVIDER")
	filter := getenv("PROVIDER_FILTER")
	if filter == "" {
		cfg.ProviderFilter = nil
	} else {
		cfg.ProviderFilter = &filter
	}
	return cfg
}
