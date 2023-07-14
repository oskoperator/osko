package v1alpha1

// +kubebuilder:object:generate=true
type Mimir struct {
	Address      string       `json:"address,omitempty"`
	Ruler        Ruler        `json:"ruler,omitempty"`
	Multitenancy Multitenancy `json:"multitenancy,omitempty"`
}
