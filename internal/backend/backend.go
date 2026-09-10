// Package backend describes the metrics backends OSKO can target and the
// capabilities each one provides. Controllers ask this package what a backend
// can do rather than comparing type strings.
//
// Note that this is not the only place backend names appear:
// internal/helpers.isPrometheusSource keeps a separate list for the SLI
// metric-source dialect, which is a different field with different values.
package backend

import (
	"fmt"
	"strings"
)

// Type identifies a metrics backend. The values match the enum accepted by
// Datasource.spec.type.
//
// Obtain a Type through Parse. The capability methods below are deliberately
// methods rather than functions over a raw string: Parse is the single
// validation boundary, so holding a Type means the unknown-backend error has
// already been handled and cannot be silently swallowed.
type Type string

const (
	Prometheus      Type = "prometheus"
	Mimir           Type = "mimir"
	Cortex          Type = "cortex"
	Thanos          Type = "thanos"
	VictoriaMetrics Type = "victoriametrics"
)

const (
	// TenantHeaderMimir scopes a request to a tenant in Mimir and Cortex.
	TenantHeaderMimir = "X-Scope-OrgID"

	// TenantHeaderThanos scopes a request to a tenant in Thanos Query. It
	// matches the default of Thanos' --query.tenant-header flag.
	TenantHeaderThanos = "THANOS-TENANT"

	// prometheusAPISubPath is where Mimir and Cortex expose the Prometheus
	// HTTP API. The others expose it at the root.
	prometheusAPISubPath = "/prometheus"
)

// Parse normalises a Datasource type string and is the only way to obtain a
// Type from user input.
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
	case VictoriaMetrics:
		return VictoriaMetrics, nil
	default:
		// Report the raw input, not the normalised form, so the operator sees
		// exactly what they typed.
		return "", fmt.Errorf("unsupported datasource type: %q", t)
	}
}

// NeedsRemoteRulePush reports whether rule groups must be pushed to a remote
// ruler configuration API.
//
// Mimir and Cortex expose one. Thanos Ruler has no rule-write API and instead
// reads files rendered by prometheus-operator from PrometheusRule objects;
// Prometheus and VictoriaMetrics work the same way. Neither needs a push.
func (t Type) NeedsRemoteRulePush() bool {
	switch t {
	case Mimir, Cortex:
		return true
	default:
		return false
	}
}

// SupportsMagicAlerting reports whether the backend exposes an Alertmanager
// configuration API that OSKO can write routing configuration to.
//
// Thanos Ruler sends alerts to an Alertmanager configured statically through
// --alertmanagers.url, which is outside OSKO's control.
func (t Type) SupportsMagicAlerting() bool {
	switch t {
	case Mimir, Cortex:
		return true
	default:
		return false
	}
}

// QueryURL returns the base URL of the Prometheus-compatible query API for a
// backend reachable at address.
//
// This is kept separate from NeedsRemoteRulePush even though the two agree on
// today's backend set: serving the query API under a sub-path and exposing a
// ruler write API are unrelated properties that coincide by accident.
func (t Type) QueryURL(address string) string {
	trimmed := strings.TrimRight(address, "/")
	switch t {
	case Mimir, Cortex:
		return trimmed + prometheusAPISubPath
	default:
		return trimmed
	}
}

// TenantHeader returns the HTTP header carrying the tenant identifier for the
// backend, or an empty string when the backend has no tenancy header.
func (t Type) TenantHeader() string {
	switch t {
	case Mimir, Cortex:
		return TenantHeaderMimir
	case Thanos:
		return TenantHeaderThanos
	default:
		return ""
	}
}
