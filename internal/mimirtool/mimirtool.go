package mimirtool

import (
	mimirtool "github.com/grafana/mimir/pkg/mimirtool/client"
)

type MimirClientConfig struct {
	Address  string
	TenantId string
}

func (m *MimirClientConfig) NewMimirClient() (*mimirtool.MimirClient, error) {
	return mimirtool.New(
		mimirtool.Config{
			Address: m.Address,
			ID:      m.TenantId,
		},
	)
}
