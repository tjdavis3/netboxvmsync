package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
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
	NetboxURL     string `env:"NETBOX_URL"`
	NetboxToken   string `env:"NETBOX_TOKEN"`
	ProviderURL   string `env:"PROVIDER_URL"`
	ProviderUser  string `env:"PROVIDER_USER"`
	ProviderToken string `env:"PROVIDER_TOKEN"`
}

func main() {
	cfg := Configure(os.Getenv)
	nb := netbox.NewClient(cfg.NetboxURL, cfg.NetboxToken, slog.Default())
	slog.Info("Created Netbox client", "url", cfg.NetboxURL)

	provider, err := vmware.NewVmwareProvider(cfg.ProviderURL, cfg.ProviderUser, cfg.ProviderToken, slog.Default())
	if err != nil {
		log.Fatal(err)
	}
	service := sync.NewSyncService(nb, provider, slog.Default())
	service.StartSync()
}

func Configure(getenv func(string) string) Config {
	cfg := Config{}
	godotenv.Load()
	cfg.NetboxURL = getenv("NETBOX_URL")
	cfg.NetboxToken = getenv("NETBOX_TOKEN")
	cfg.ProviderURL = getenv("PROVIDER_URL")
	cfg.ProviderUser = getenv("PROVIDER_USER")
	cfg.ProviderToken = getenv("PROVIDER_TOKEN")
	return cfg
}
