package common

type Description string

// +kubebuilder:validation:Pattern=`^[1-9]\d*[m h d]`
type Duration string
