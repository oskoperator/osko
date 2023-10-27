# Output metric naming

* Status: proposed
* Date: 2023-10-27

## Context and Problem Statement

The solution provided by the operator doesn't end with a `PrometheusRule` CRD, but takes into account as much of the SLO journey as possible. 
Part of that journey is displaying Indicators, Objectives, Error Budgets etc. on different dashboards. The design should account for
existence of reusable Grafana dashboards at the very least.

## Considered Options

* Prometheus Recording Rule naming convention
  * With part of slo in name: `slo:logging_query_frontend_error_rate:error_budget:28d`
  * With osko in name: `osko:error_budget{slo_name="logging_query_frontend_error_rate", window="28d", ...}`
* Pretend to be a metric
  * `osko_error_budget{slo_name="logging_query_frontend_error_rate", window="28d", ...}`

## Decision Outcome

Chosen option: "Pretend to be a metric". There is a reasonable chance that `PrometheusRule` CRD might not be the only possible output
of OSKO and thus it's worth it to avoid the result being too implementation specific.

The metric names that should be exposed/recorded:

* `osko_sli_ratio_good`
* `osko_sli_ratio_bad`
* `osko_sli_ratio_total`
* `osko_sli_measurement`
* `osko_slo_target`
* `osko_error_budget_available`

Ideally also the following, although these might be implementation specific (aka we'll see how hard it is):

* `osko_error_budget_burn_rate`
* `osko_error_budget_burn_rate_threshold`

The following labels should be present next to the metric (where applicable):

*Note that the following might change as we explore implementing composite SLOs and other more complex scenarios.*

* `sli_name`
* `slo_name`
* `service`
* `window`

### Positive Consequences

* Future implementations will be able to plug in to the rest of the solution
* Exhaustive list of metrics will make creation of the dashboards easy

### Negative Consequences

* Tracing resulting metric back to the original will be only possible by looking at the SLI specification manifest.

## Pros and Cons of the Options

### Prometheus Recording Rule naming convention - With part of slo in name

`slo:loki_request_duration_seconds_count:ratio_good:28d`

* Good, because it can show the original metric
* Bad, because dashboards would have to be generated for each metric

### Prometheus Recording Rule naming convention -  With osko in name

`osko:error_budget{slo_name="logging_query_frontend_error_rate", window="28d", ...}`

* Good, because it allows for reusable dashboards
* Bad, because it's tightly tied to Prometheus recording rule implementation

## Links 

* [Prometheus recordingrule naming recommendations](https://prometheus.io/docs/practices/rules/#naming)
