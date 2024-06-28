package netbox

import (
	"github.com/ringsq/netboxvmsync/pkg/sync"
)

type VMSearchResponse struct {
	Count    int              `json:"count"`
	Next     *string          `json:"next"`
	Previous *string          `json:"previous"`
	Results  []VMSearchResult `json:"results"`
}

type VMSearchResult struct {
	ID          int64           `json:"id"`
	URL         string          `json:"url"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Cluster     EmbeddedCluster `json:"cluster"`
}

type EmbeddedCluster struct {
	ID   int64  `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type DerivedCluster struct {
	Cluster sync.Cluster
	GroupID string
}

func (d DerivedCluster) GetID() string {
	return d.Cluster.ID
}
