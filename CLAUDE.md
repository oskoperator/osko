# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OSKO is a Kubernetes operator for managing SLIs (Service Level Indicators), SLOs (Service Level Objectives), alerting rules, and alert routing via Kubernetes CRDs according to the OpenSLO specification. It aims to provide simple management of observability concepts in Kubernetes environments, with particular focus on Prometheus/Mimir-based monitoring stacks.

## Common Development Commands

### Building and Development
- `make build` - Build the manager binary
- `make run` - Run the controller locally (requires K8s cluster context)
- `make run-pretty-debug` - Run with debug output and pretty formatting using zap-pretty
- `make install run` - Install CRDs and run controller in one step

### Code Generation and Manifests
- `make manifests` - Generate CRDs, RBAC, and webhook configurations
- `make generate` - Generate DeepCopy methods for API types
- `make fmt` - Format Go code
- `make vet` - Run go vet

### Testing
- `make test` - Run all tests (includes manifests, generate, fmt, vet, and test execution)
- `KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./...` - Run tests directly

### Deployment
- `make install` - Install CRDs into current K8s context
- `make uninstall` - Remove CRDs from cluster
- `make deploy` - Deploy controller to cluster
- `make undeploy` - Remove controller from cluster
- `make docker-build` - Build Docker image
- `make docker-push` - Push Docker image

### Development Environment
- `make deploydev` - Deploy development stack (Grafana, Mimir) with port-forwards
- `make undeploydev` - Clean up development environment

## Architecture

### API Groups and Versions
- **openslo.com/v1**: Core OpenSLO specification resources (SLO, SLI, Datasource, AlertPolicy, etc.)
- **osko.dev/v1alpha1**: Operator-specific resources (MimirRule, AlertManagerConfig, etc.)

### Controller Structure
Controllers are organized by API group:
- `internal/controller/openslo/`: Controllers for OpenSLO resources
- `internal/controller/osko/`: Controllers for operator-specific resources
- `internal/controller/monitoring.coreos.com/`: Controllers for Prometheus Operator resources

### Key Controllers
- **SLO Controller** (`slo_controller.go`): Main controller implementing ownership model, creates PrometheusRules, MimirRules, inline SLIs, and AlertManagerConfigs
- **SLI Controller**: Manages Service Level Indicators
- **Datasource Controller**: Manages data source connections (Mimir, Cortex)
- **MimirRule Controller**: Manages Mimir-specific rule configurations
- **AlertManagerConfig Controller**: Manages AlertManager routing configurations

### Ownership Model
OSKO implements a comprehensive ownership model:
- **Owned Resources** (cascading deletion): inline SLIs, PrometheusRules, MimirRules, AlertManagerConfigs
- **Referenced Resources** (preserved): shared Datasources, referenced SLIs, AlertPolicies
- Uses Kubernetes finalizers for proper cleanup of external system resources

### Resource Dependencies
```
SLO -> SLI (inline or referenced) -> Datasource
SLO -> PrometheusRule (owned)
SLO -> MimirRule (owned)
SLO -> AlertManagerConfig (owned, when magicAlerting enabled)
```

## Key Directories

- `api/`: API type definitions for both openslo.com and osko.dev groups
- `internal/controller/`: Controller implementations
- `internal/helpers/`: Helper utilities for Prometheus and Mimir integration
- `internal/config/`: Configuration management
- `config/`: Kubernetes manifests (CRDs, RBAC, deployment configs)
- `helm/`: Helm charts for deployment
- `examples/`: Example resource manifests
- `docs/`: Additional documentation

## Important Implementation Notes

### Testing Requirements
- Always run `make test` before submitting changes
- Tests require KUBEBUILDER_ASSETS to be set up (handled automatically by make test)
- Integration tests exist for the ownership model in `slo_controller_test.go`

### Development Dependencies
- Requires Prometheus Operator CRDs: `helm install prometheus-operator-crds prometheus-community/prometheus-operator-crds`
- Uses controller-runtime framework
- Built with Kubebuilder

### Magic Alerting
SLOs can enable automatic AlertManager configuration via the `osko.dev/magicAlerting: "true"` annotation, which creates owned AlertManagerConfig resources for alert routing.

### Inline vs Referenced SLIs
- **Inline SLIs** (defined in `spec.indicator`): Created and owned by the SLO
- **Referenced SLIs** (defined via `spec.indicatorRef`): External resources that are referenced but not owned

### External Systems Integration
- **Mimir/Cortex**: Via MimirRule controller and connection details
- **Prometheus**: Via PrometheusRule resources compatible with prometheus-operator
- **AlertManager**: Via AlertManagerConfig for routing configuration
