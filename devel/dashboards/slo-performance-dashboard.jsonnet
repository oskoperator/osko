local grafana = import 'vendor/github.com/grafana/grafonnet-lib/grafonnet/grafana.libsonnet';
local dashboard = grafana.dashboard;
local template = grafana.template;
local statPanel = grafana.statPanel;

// Template variables
local templates = [
  template.datasource(
    'datasource',
    'prometheus',
    'Prometheus',
    hide='label',
  ),
  template.new(
    'slo_name',
    '${datasource}',
    'label_values(osko_slo_target,slo_name)',
    label='SLO',
    refresh='load',
    includeAll=false,
    multi=false,
  ),
  template.new(
    'service',
    '${datasource}',
    'label_values(osko_slo_target{slo_name="$slo_name"},service)',
    label='Service',
    refresh='load',
    includeAll=false,
    multi=false,
  ),
  template.new(
    'window',
    '${datasource}',
    'label_values(osko_sli_measurement{slo_name="$slo_name", service="$service"},window)',
    label='Window',
    refresh='load',
    includeAll=false,
    multi=false,
  ),
];

// 1. SLI Status - Current success rate
local sliStatusPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  fieldConfig: {
    defaults: {
      decimals: 2,
      max: 100,
      min: 0,
      thresholds: {
        mode: 'absolute',
        steps: [
          { color: 'red', value: 0 },
          { color: 'yellow', value: 98.5 },
          { color: 'green', value: 99 },
        ],
      },
      unit: 'percent',
    },
  },
  gridPos: { h: 6, w: 8, x: 0, y: 0 },
  id: 1,
  options: {
    colorMode: 'background',
    textMode: 'auto',
    wideLayout: true,
  },
  targets: [{
    expr: 'osko_sli_measurement{slo_name="$slo_name", service="$service", window="$window"} * 100',
    instant: true,
    legendFormat: 'SLI Success Rate',
  }],
  title: 'SLI Status (Current)',
  type: 'stat',
};

// 2. Error Budget Remaining - 28d cumulative
local errorBudgetPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  fieldConfig: {
    defaults: {
      decimals: 1,
      max: 100,
      min: 0,
      thresholds: {
        mode: 'absolute',
        steps: [
          { color: 'red', value: 0 },
          { color: 'yellow', value: 25 },
          { color: 'green', value: 50 },
        ],
      },
      unit: 'percent',
    },
  },
  gridPos: { h: 6, w: 16, x: 8, y: 0 },
  id: 2,
  options: {
    orientation: 'horizontal',
    displayMode: 'basic',
    showUnfilled: true,
  },
  targets: [{
    expr: '(1 - osko_error_budget_value{slo_name="$slo_name", service="$service", window="$window"}) * 100',
    instant: true,
    legendFormat: 'Error Budget Remaining (28d)',
  }],
  title: 'Error Budget Remaining',
  type: 'bargauge',
};

// 3. SLI Trend - Success rate over time
local sliTrendPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  fieldConfig: {
    defaults: {
      max: 100,
      min: 98,
      unit: 'percent',
      custom: {
        lineWidth: 2,
        fillOpacity: 10,
      },
    },
    overrides: [{
      matcher: { id: 'byName', options: 'SLO Target' },
      properties: [
        { id: 'color', value: { mode: 'fixed', fixedColor: 'red' } },
        { id: 'custom.lineStyle', value: { dash: [10, 10] } },
      ],
    }],
  },
  gridPos: { h: 8, w: 12, x: 0, y: 6 },
  id: 3,
  options: {
    legend: { displayMode: 'table', placement: 'bottom' },
  },
  targets: [
    {
      expr: 'osko_sli_measurement{slo_name="$slo_name", service="$service", window="$window"} * 100',
      legendFormat: 'SLI Success Rate',
    },
    {
      expr: 'osko_slo_target{slo_name="$slo_name", service="$service"} * 100',
      legendFormat: 'SLO Target (99%)',
    },
  ],
  title: 'SLI Trend',
  type: 'timeseries',
};

// 4. Error Budget Burndown - Cumulative consumption over 28d
local burndownPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  fieldConfig: {
    defaults: {
      max: 100,
      min: 0,
      unit: 'percent',
      custom: {
        lineWidth: 2,
        fillOpacity: 20,
      },
    },
  },
  gridPos: { h: 8, w: 12, x: 12, y: 6 },
  id: 4,
  options: {
    legend: { displayMode: 'table', placement: 'bottom' },
  },
  targets: [{
    expr: '(1 - osko_error_budget_value{slo_name="$slo_name", service="$service", window="$window"}) * 100',
    legendFormat: 'Error Budget Remaining ($window)',
  }],
  title: 'Error Budget Burndown (28d)',
  type: 'timeseries',
};

// 5. Query Latency Distribution
local latencyDistPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  fieldConfig: {
    defaults: {
      unit: 's',
      custom: {
        drawStyle: 'bars',
        fillOpacity: 80,
      },
    },
  },
  gridPos: { h: 8, w: 12, x: 0, y: 14 },
  id: 5,
  options: {
    legend: { displayMode: 'table', placement: 'bottom' },
  },
  targets: [{
    expr: 'histogram_quantile(0.50, sum(rate(cortex_distributor_query_duration_seconds_bucket{method="Distributor.QueryStream"}[5m])) by (le))',
    legendFormat: 'p50',
  }, {
    expr: 'histogram_quantile(0.95, sum(rate(cortex_distributor_query_duration_seconds_bucket{method="Distributor.QueryStream"}[5m])) by (le))',
    legendFormat: 'p95',
  }, {
    expr: 'histogram_quantile(0.99, sum(rate(cortex_distributor_query_duration_seconds_bucket{method="Distributor.QueryStream"}[5m])) by (le))',
    legendFormat: 'p99',
  }],
  title: 'Query Latency Percentiles',
  type: 'timeseries',
};

// 6. Burn Rate Alert
local burnRatePanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  fieldConfig: {
    defaults: {
      min: 0,
      unit: 'short',
      thresholds: {
        mode: 'absolute',
        steps: [
          { color: 'green', value: 0 },
          { color: 'yellow', value: 2 },
          { color: 'red', value: 5 },
        ],
      },
      custom: {
        drawStyle: 'bars',
        fillOpacity: 100,
      },
    },
  },
  gridPos: { h: 8, w: 12, x: 12, y: 14 },
  id: 6,
  options: {
    legend: { displayMode: 'table', placement: 'bottom' },
  },
  targets: [{
    expr: 'osko_error_budget_burn_rate{slo_name="$slo_name", service="$service", window="$window"}',
    legendFormat: 'Current Burn Rate',
  }],
  title: 'Error Budget Burn Rate',
  type: 'timeseries',
};

// Dashboard definition
dashboard.new(
  title='SLO Performance Dashboard - Latency',
  uid='slo-performance',
  tags=['slo', 'latency', 'mimir', 'performance'],
  time_from='now-6h',
  time_to='now',
  refresh='30s',
  editable=true,
  graphTooltip='shared_crosshair',
)
.addTemplates(templates)
.addPanels([
  sliStatusPanel,
  errorBudgetPanel,
  sliTrendPanel,
  burndownPanel,
  latencyDistPanel,
  burnRatePanel,
]) + {
  fiscalYearStartMonth: 0,
  preload: false,
  schemaVersion: 41,
}
