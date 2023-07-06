package v1alpha1

type Mimir struct {
	Address      string       `json:"address,omitempty"`
	Ruler        Ruler        `json:"ruler,omitempty"`
	Multitenancy Multitenancy `json:"multitenancy,omitempty"`
}
