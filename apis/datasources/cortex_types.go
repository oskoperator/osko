package v1alpha1

type Cortex struct {
	Address      string       `json:"address,omitempty"`
	Ruler        Ruler        `json:"ruler,omitempty"`
	Multitenancy Multitenancy `json:"multitenancy,omitempty"`
}
