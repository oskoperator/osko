package v1

type Duration struct {
	// +kubebuilder:validation:Pattern=`^[1-9]\d*[m h d]`
	Duration string `json:"duration:omitempty"`
}
