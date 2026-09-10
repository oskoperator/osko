// Package backend describes the metrics backends OSKO can target and the
// capabilities each one provides. Controllers ask this package what a backend
// can do rather than comparing type strings, so adding a new backend means
// changing one file.
package backend

import (
	"fmt"
	"strings"
)

// Type identifies a metrics backend. The values match the enum accepted by
// Datasource.spec.type.
type Type string

const (
	Prometheus Type = "prometheus"
	Mimir      Type = "mimir"
	Cortex     Type = "cortex"
	Thanos     Type = "thanos"
)

const (
	// TenantHeaderMimir scopes a request to a tenant in Mimir and Cortex.
	TenantHeaderMimir = "X-Scope-OrgID"

	// TenantHeaderThanos scopes a request to a tenant in Thanos Query. It
	// matches the default of Thanos' --query.tenant-header flag.
	TenantHeaderThanos = "THANOS-TENANT"

	// prometheusAPISubPath is where Mimir and Cortex expose the Prometheus
	// HTTP API. Thanos and Prometheus expose it at the root.
	prometheusAPISubPath = "/prometheus"
)

// Parse normalises a Datasource type string.
//
// Matching is case-insensitive on purpose: the CRD enum only validates on
// write, so a Datasource stored before the enum was introduced can still be
// read back with mixed case.
func Parse(t string) (Type, error) {
	switch Type(strings.ToLower(strings.TrimSpace(t))) {
	case Prometheus:
		return Prometheus, nil
	case Mimir:
		return Mimir, nil
	case Cortex:
		return Cortex, nil
	case Thanos:
		return Thanos, nil
	default:
		return "", fmt.Errorf("unsupported datasource type: %q", t)
	}
}

// NeedsRemoteRulePush reports whether rule groups must be pushed to a remote
// ruler configuration API.
//
// Mimir and Cortex expose one. Thanos Ruler has no rule-write API and instead
// reads files rendered by prometheus-operator from PrometheusRule objects;
// Prometheus works the same way. Neither needs a push.
func NeedsRemoteRulePush(t string) bool {
	parsed, err := Parse(t)
	if err != nil {
		return false
	}
	return parsed == Mimir || parsed == Cortex
}

// SupportsMagicAlerting reports whether the backend exposes an Alertmanager
// configuration API that OSKO can write routing configuration to.
//
// Thanos Ruler sends alerts to an Alertmanager configured statically through
// --alertmanagers.url, which is outside OSKO's control.
func SupportsMagicAlerting(t string) bool {
	parsed, err := Parse(t)
	if err != nil {
		return false
	}
	return parsed == Mimir || parsed == Cortex
}

// QueryURL returns the base URL of the Prometheus-compatible query API for the
// backend reachable at address.
func QueryURL(t, address string) (string, error) {
	parsed, err := Parse(t)
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSuffix(address, "/")
	if parsed == Mimir || parsed == Cortex {
		return trimmed + prometheusAPISubPath, nil
	}
	return trimmed, nil
}

// TenantHeader returns the HTTP header carrying the tenant identifier for the
// backend, or an empty string when the backend has no tenancy header.
func TenantHeader(t string) string {
	parsed, err := Parse(t)
	if err != nil {
		return ""
	}

	switch parsed {
	case Mimir, Cortex:
		return TenantHeaderMimir
	case Thanos:
		return TenantHeaderThanos
	default:
		return ""
	}
}
