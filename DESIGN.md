# osko - OpenSLO Kubernetes Operator

## Introduction

This operator aims to provide it's users with simple management of SLIs, SLOs, alerting rules and alerts routing via Kubernetes CRDs according to the (not only) the [OpenSLO](https://github.com/OpenSLO/OpenSLO) specification (currently `v1`).

## Goals

The goals of the operator are to take **inputs** in the form of metrics from supported datasources (`Mimir`, `Cortex` for a start) and produce **outputs** in the form of Prometheus `rules` in the form of the [`PrometheusRule`](https://prometheus-operator.dev/docs/operator/design/#prometheusrule) CRDs based on set `OpenSLO`s.

### Example inputs and outputs

#### Inputs

[kind: Datasource `spec.connectionDetails`](https://github.com/oskoperator/osko/blob/main/apis/openslo/v1/datasource_types.go#L11)

#### Outputs

[`PrometheusRule`](https://prometheus-operator.dev/docs/operator/design/#prometheusrule)

If the target system is unable to reconcile the created [`PrometheusRule`](https://prometheus-operator.dev/docs/operator/design/#prometheusrule)s on it's own (like [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator)), we allow extending our operator with controllers that will be able to reconcile [`PrometheusRule`](https://prometheus-operator.dev/docs/operator/design/#prometheusrule)s against specific target systems (any arbitrary API, for example [`Cortex`s Ruler](https://cortexmetrics.io/docs/api/#ruler)).

## Non-Goals

- Support the full OpenSLO specification from the get-go, **if ever**
  - The goal here is to be _compatible_ with the OpenSLO spec, not necessarily fully implement it
  - MRs (PRs) are welcome for any missing functionality.

# Technical notes

- We should look into how to implement [Multiwindow, Multi-Burn-Rate Alerts](https://sre.google/workbook/alerting-on-slos/#6-multiwindow-multi-burn-rate-alerts) based on the OpenSLO spec

## Design

```mermaid
---
Title: OSKO Dependency Graph
---
flowchart LR;
subgraph userspace
sloObject(SLO)
sliObject(SLI)
dataSourceObject(DataSource)
end
subgraph controllerspace
prometheusRuleObject(PrometheusRule)
end

sloController(SLO Controller)
mimirRuleController(Mimir Rule Controller)
sliController(SLI Controller)
dataSourceController(DataSource Controller)

subgraph external
mimir[Mimir]
cortex[Cortex]
end

cortexRuleController(Optional: Cortex Rule Controller)
cortexRuleController --> |Watch| prometheusRuleObject
cortexRuleController --> |Updates| cortex

mimirRuleController --> |Watch| prometheusRuleObject
mimirRuleController --> |Updates| mimir

sloController --> |Own| sloObject
sloController --> |Watch| sliObject
sloController --> |Watch| dataSourceObject
sloController --> |Own| prometheusRuleObject

sliController --> |Own| sliObject
sliController --> |Watch| dataSourceObject

dataSourceController --> |Own| dataSourceObject

sloObject --> |Reference| sliObject
sliObject --> |Reference| dataSourceObject
%% reference slo -> datasource asi netreba, to bereme na zaklade SLIs ne? Dela to pak
%% hnusnej graf :D, kdyztak zkus odkomentovat
%%  sloObject --> |Reference| dataSourceObject
%%  prometheusRuleObject --> |Reference| dataSourceObject
```

## Resource Lifecycle

```mermaid
flowchart TD
    subgraph "User-Created Resources"
        DS[Datasource]
        SLI[SLI]
        SLO[SLO]
    end

    subgraph "Controller-Created Resources"
        PR[PrometheusRule]
        MR[MimirRule]
        AMC[AlertManagerConfig]
    end

    subgraph "External Systems"
        Mimir[(Mimir/Cortex)]
        AlertManager[(AlertManager)]
    end

    %% User resource relationships
    SLO -->|references| SLI
    SLO -->|references via annotation| DS
    SLI -->|references| DS

    %% Controller relationships
    SLOController[SLO Controller]
    MimirRuleController[MimirRule Controller]
    AMCController[AlertManagerConfig Controller]
    PRController[PrometheusRule Controller]

    %% Controller watches and creates
    SLOController -->|watches| SLO
    SLOController -->|creates/owns| PR
    SLOController -->|creates/owns| MR

    MimirRuleController -->|watches| MR
    MimirRuleController -->|submits rules to| Mimir

    AMCController -->|watches| AMC
    AMCController -->|submits config to| AlertManager

    PRController -->|watches| PR

    %% Resource lifecycle with finalizers
    SLO -->|deletes with finalizer| PR
    SLO -->|deletes with finalizer| MR

    MR -->|has finalizer| MimirRule_Finalizer[MimirRule Finalizer]
    MimirRule_Finalizer -->|cleanup rules in| Mimir

    AMC -->|has finalizer| AMC_Finalizer[AlertManagerConfig Finalizer]
    AMC_Finalizer -->|cleanup config in| AlertManager

    %% Status updates
    SLOController -->|updates status| SLO
    MimirRuleController -->|updates status| MR
    AMCController -->|updates status| AMC

    %% Legend
    classDef userResource fill:#b7e1cd,stroke:#82b366
    classDef controllerResource fill:#d0e0e3,stroke:#6c8ebf
    classDef controller fill:#ffe6cc,stroke:#d79b00
    classDef external fill:#f5f5f5,stroke:#666666
    classDef finalizer fill:#fff2cc,stroke:#d6b656

    class DS,SLI,SLO userResource
    class PR,MR,AMC controllerResource
    class SLOController,MimirRuleController,AMCController,PRController controller
    class Mimir,AlertManager external
    class MimirRule_Finalizer,AMC_Finalizer finalizer
```
