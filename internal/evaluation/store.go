package evaluation

import (
	"statsig/internal/net"
)

type Store struct {
	FeatureGates   map[string]net.ConfigSpec
	DynamicConfigs map[string]net.ConfigSpec
	LastSyncTime   int64
}

func initStore(n *net.Net) *Store {
	store := &Store{
		FeatureGates:   make(map[string]net.ConfigSpec),
		DynamicConfigs: make(map[string]net.ConfigSpec),
	}

	specs := n.FetchConfigSpecs()
	if specs.HasUpdates {
		newGates := make(map[string]net.ConfigSpec)
		for _, gate := range specs.FeatureGates {
			newGates[gate.Name] = gate
		}

		newConfigs := make(map[string]net.ConfigSpec)
		for _, config := range specs.FeatureGates {
			newConfigs[config.Name] = config
		}

		store.FeatureGates = newGates
		store.DynamicConfigs = newConfigs
	}

	return store
}
