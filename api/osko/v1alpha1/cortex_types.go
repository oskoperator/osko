package v1alpha1

// +kubebuilder:object:generate=true
type Cortex struct {
	Address      string       `json:"address,omitempty"`
	Ruler        Ruler        `json:"ruler,omitempty"`
	Multitenancy Multitenancy `json:"multitenancy,omitempty"`
}
