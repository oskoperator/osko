local grafana = import 'vendor/github.com/grafana/grafonnet-lib/grafonnet/grafana.libsonnet';
local dashboard = grafana.dashboard;
local template = grafana.template;
local row = grafana.row;

// Template variables with All options and cascading filters
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
    includeAll=true,
    multi=false,
  ),
  template.new(
    'service',
    '${datasource}',
    'label_values(osko_slo_target{slo_name=~"$slo_name"},service)',
    label='Service',
    refresh='load',
    includeAll=true,
    multi=false,
  ),
  template.new(
    'window',
    '${datasource}',
    'label_values(osko_sli_measurement{slo_name=~"$slo_name", service=~"$service"},window)',
    label='Window',
    refresh='load',
    includeAll=true,
    multi=false,
  ),
];

// ============ ROW 1: SLO STATUS OVERVIEW ============
// Row header for status overview section
local statusRow = {
  collapsed: false,
  gridPos: { h: 1, w: 24, x: 0, y: 0 },
  id: 10,
  panels: [],
  title: 'SLO Status at a Glance',
  type: 'row',
};

// 1. SLO Compliance Status - Boolean pass/fail indicator
local compliancePanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    Current SLO compliance status. 
    Green = SLI is meeting the SLO target.
    Red = SLI is below the SLO target.
  |||,
  fieldConfig: {
    defaults: {
      thresholds: {
        mode: 'absolute',
        steps: [
          { color: 'red', value: 0 },
          { color: 'green', value: 1 },
        ],
      },
      unit: 'bool',
      mappings: [
        { options: { '0': { text: 'FAILING', color: 'red' } }, type: 'value' },
        { options: { '1': { text: 'PASSING', color: 'green' } }, type: 'value' },
      ],
    },
  },
  gridPos: { h: 6, w: 6, x: 0, y: 1 },
  id: 1,
  options: {
    colorMode: 'background',
    textMode: 'value',
    wideLayout: true,
    graphMode: 'none',
  },
  targets: [{
    expr: 'osko_sli_measurement{slo_name=~"$slo_name", service=~"$service", window=~"$window"} >= osko_slo_target{slo_name=~"$slo_name", service=~"$service"}',
    instant: true,
    legendFormat: 'SLO Compliance',
  }],
  title: 'SLO Compliance',
  type: 'stat',
};

// 2. SLI Status - Current success rate with better thresholds
local sliStatusPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    Current Service Level Indicator (SLI) value as a percentage.
    This shows the actual performance against the target.
    Green = Meeting SLO, Yellow = At risk, Red = Breaching SLO.
  |||,
  fieldConfig: {
    defaults: {
      decimals: 3,
      max: 100,
      min: 0,
      thresholds: {
        mode: 'absolute',
        steps: [
          { color: 'red', value: 0 },
          { color: 'yellow', value: 95 },
          { color: 'green', value: 99 },
        ],
      },
      unit: 'percent',
    },
  },
  gridPos: { h: 6, w: 6, x: 6, y: 1 },
  id: 2,
  options: {
    colorMode: 'background',
    textMode: 'auto',
    wideLayout: true,
    graphMode: 'area',
  },
  targets: [{
    expr: 'osko_sli_measurement{slo_name=~"$slo_name", service=~"$service", window=~"$window"} * 100',
    instant: true,
    legendFormat: 'SLI Success Rate',
  }],
  title: 'Current SLI',
  type: 'stat',
};

// 3. SLO Target Reference
local sloTargetPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: 'The target SLO value that must be maintained.',
  fieldConfig: {
    defaults: {
      decimals: 3,
      max: 100,
      min: 0,
      unit: 'percent',
      color: { mode: 'fixed', fixedColor: 'blue' },
    },
  },
  gridPos: { h: 6, w: 4, x: 12, y: 1 },
  id: 3,
  options: {
    colorMode: 'background',
    textMode: 'auto',
    wideLayout: true,
    graphMode: 'none',
  },
  targets: [{
    expr: 'osko_slo_target{slo_name=~"$slo_name", service=~"$service"} * 100',
    instant: true,
    legendFormat: 'SLO Target',
  }],
  title: 'SLO Target',
  type: 'stat',
};

// 4. Error Budget Remaining - Improved gauge
local errorBudgetPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    Percentage of error budget remaining for the current window.
    Red = <25% remaining (critical), Yellow = <50% remaining (warning), Green = >50% remaining (healthy).
  |||,
  fieldConfig: {
    defaults: {
      decimals: 1,
      max: 100,
      min: 0,
      thresholds: {
        mode: 'absolute',
        steps: [
          { color: 'red', value: 0 },
          { color: 'orange', value: 10 },
          { color: 'yellow', value: 25 },
          { color: 'green', value: 50 },
        ],
      },
      unit: 'percent',
    },
  },
  gridPos: { h: 6, w: 8, x: 16, y: 1 },
  id: 4,
  options: {
    orientation: 'auto',
    displayMode: 'lcd',
    showUnfilled: true,
    minVizWidth: 0,
    minVizHeight: 0,
  },
  targets: [{
    expr: '(1 - osko_error_budget_value{slo_name=~"$slo_name", service=~"$service", window=~"$window"}) * 100',
    instant: true,
    legendFormat: 'Error Budget Remaining',
  }],
  title: 'Error Budget Remaining',
  type: 'gauge',
};

// ============ ROW 2: TRENDS OVER TIME ============
local trendsRow = {
  collapsed: false,
  gridPos: { h: 1, w: 24, x: 0, y: 7 },
  id: 20,
  panels: [],
  title: 'Trends & History',
  type: 'row',
};

// 5. SLI Trend - Success rate over time with better visualization
local sliTrendPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    Historical trend of SLI performance over time.
    The red dashed line shows the SLO target for reference.
  |||,
  fieldConfig: {
    defaults: {
      min: 90,
      max: 100,
      unit: 'percent',
      custom: {
        lineWidth: 2,
        fillOpacity: 10,
        showPoints: 'never',
        spanNulls: false,
      },
    },
    overrides: [
      {
        matcher: { id: 'byName', options: 'SLO Target' },
        properties: [
          { id: 'color', value: { mode: 'fixed', fixedColor: 'red' } },
          { id: 'custom.lineStyle', value: { dash: [10, 10], fill: 'dash' } },
          { id: 'custom.lineWidth', value: 2 },
        ],
      },
    ],
  },
  gridPos: { h: 8, w: 12, x: 0, y: 8 },
  id: 5,
  options: {
    legend: { 
      displayMode: 'table', 
      placement: 'bottom',
      calcs: ['mean', 'min', 'max'],
    },
    tooltip: { mode: 'multi', sort: 'none' },
  },
  targets: [
    {
      expr: 'osko_sli_measurement{slo_name=~"$slo_name", service=~"$service", window=~"$window"} * 100',
      legendFormat: 'SLI Success Rate',
    },
    {
      expr: 'osko_slo_target{slo_name=~"$slo_name", service=~"$service"} * 100',
      legendFormat: 'SLO Target',
    },
  ],
  title: 'SLI Trend Over Time',
  type: 'timeseries',
};

// 6. Error Budget Burndown - Cumulative consumption
local burndownPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    Error budget consumption over time.
    Shows how much budget has been used vs remaining.
    A steady decline is expected; rapid drops indicate incidents.
  |||,
  fieldConfig: {
    defaults: {
      max: 100,
      min: 0,
      unit: 'percent',
      custom: {
        lineWidth: 2,
        fillOpacity: 20,
        showPoints: 'never',
        gradientMode: 'opacity',
      },
    },
  },
  gridPos: { h: 8, w: 12, x: 12, y: 8 },
  id: 6,
  options: {
    legend: { 
      displayMode: 'table', 
      placement: 'bottom',
      calcs: ['mean', 'min', 'lastNotNull'],
    },
    tooltip: { mode: 'multi', sort: 'none' },
  },
  targets: [{
    expr: '(1 - osko_error_budget_value{slo_name=~"$slo_name", service=~"$service", window=~"$window"}) * 100',
    legendFormat: 'Error Budget Remaining ({{window}})',
  }],
  title: 'Error Budget Burndown',
  type: 'timeseries',
};

// ============ ROW 3: BURN RATE ANALYSIS ============
local burnRateRow = {
  collapsed: false,
  gridPos: { h: 1, w: 24, x: 0, y: 16 },
  id: 30,
  panels: [],
  title: 'Burn Rate Analysis',
  type: 'row',
};

// 7. Error Budget Burn Rate - Changed from bars to lines with threshold lines
local burnRatePanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    How quickly the error budget is being consumed.
    - Green line = Fast burn threshold (14.4x) - page immediately
    - Yellow line = Slow burn threshold (2x) - ticket for investigation
    Values above these lines indicate excessive budget consumption.
  |||,
  fieldConfig: {
    defaults: {
      min: 0,
      unit: 'short',
      custom: {
        drawStyle: 'line',
        lineInterpolation: 'linear',
        lineWidth: 2,
        fillOpacity: 0,
        showPoints: 'never',
        spanNulls: false,
      },
    },
    overrides: [
      {
        matcher: { id: 'byName', options: 'Fast Burn Threshold (14.4x)' },
        properties: [
          { id: 'color', value: { mode: 'fixed', fixedColor: 'red' } },
          { id: 'custom.lineStyle', value: { dash: [10, 10], fill: 'dash' } },
          { id: 'custom.lineWidth', value: 2 },
        ],
      },
      {
        matcher: { id: 'byName', options: 'Slow Burn Threshold (2x)' },
        properties: [
          { id: 'color', value: { mode: 'fixed', fixedColor: 'yellow' } },
          { id: 'custom.lineStyle', value: { dash: [5, 5], fill: 'dash' } },
          { id: 'custom.lineWidth', value: 2 },
        ],
      },
      {
        matcher: { id: 'byName', options: 'Current Burn Rate' },
        properties: [
          { id: 'color', value: { mode: 'fixed', fixedColor: 'blue' } },
          { id: 'custom.lineWidth', value: 3 },
        ],
      },
    ],
  },
  gridPos: { h: 8, w: 12, x: 0, y: 17 },
  id: 7,
  options: {
    legend: { 
      displayMode: 'table', 
      placement: 'bottom',
      calcs: ['mean', 'max', 'lastNotNull'],
    },
    tooltip: { mode: 'multi', sort: 'desc' },
  },
  targets: [
    {
      expr: 'osko_error_budget_burn_rate{slo_name=~"$slo_name", service=~"$service", window=~"$window"}',
      legendFormat: 'Current Burn Rate',
    },
    {
      expr: 'osko_error_budget_burn_rate_threshold{slo_name=~"$slo_name", service=~"$service", window=~"$window"}',
      legendFormat: 'Fast Burn Threshold (14.4x)',
    },
    {
      expr: 'osko_error_budget_burn_rate_threshold{slo_name=~"$slo_name", service=~"$service", window=~"$window"} / 7.2',
      legendFormat: 'Slow Burn Threshold (2x)',
    },
  ],
  title: 'Error Budget Burn Rate',
  type: 'timeseries',
};

// 8. Events Summary - Shows good vs bad events
local eventsSummaryPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    Summary of events contributing to the SLI calculation.
    Shows total events and helps understand the volume behind the SLI.
  |||,
  fieldConfig: {
    defaults: {
      unit: 'short',
      custom: {
        lineWidth: 2,
        fillOpacity: 10,
        showPoints: 'never',
        stack: { mode: 'normal', group: 'A' },
      },
    },
    overrides: [
      {
        matcher: { id: 'byName', options: 'Good Events' },
        properties: [
          { id: 'color', value: { mode: 'fixed', fixedColor: 'green' } },
        ],
      },
      {
        matcher: { id: 'byName', options: 'Bad Events' },
        properties: [
          { id: 'color', value: { mode: 'fixed', fixedColor: 'red' } },
        ],
      },
    ],
  },
  gridPos: { h: 8, w: 12, x: 12, y: 17 },
  id: 8,
  options: {
    legend: { 
      displayMode: 'table', 
      placement: 'bottom',
      calcs: ['sum'],
    },
    tooltip: { mode: 'multi', sort: 'none' },
  },
  targets: [
    {
      expr: 'osko_sli_total{slo_name=~"$slo_name", service=~"$service", window=~"$window"} * osko_sli_measurement{slo_name=~"$slo_name", service=~"$service", window=~"$window"}',
      legendFormat: 'Good Events',
    },
    {
      expr: 'osko_sli_total{slo_name=~"$slo_name", service=~"$service", window=~"$window"} * (1 - osko_sli_measurement{slo_name=~"$slo_name", service=~"$service", window=~"$window"})',
      legendFormat: 'Bad Events',
    },
  ],
  title: 'Good vs Bad Events',
  type: 'timeseries',
};

// ============ ROW 4: MULTI-SLO COMPARISON ============
local comparisonRow = {
  collapsed: true,
  gridPos: { h: 1, w: 24, x: 0, y: 25 },
  id: 40,
  panels: [],
  title: 'Multi-SLO Comparison (Expand when viewing All)',
  type: 'row',
};

// 9. SLO Comparison Table - Shows all SLOs when viewing All
local sloComparisonPanel = {
  datasource: { type: 'prometheus', uid: '$datasource' },
  description: |||
    Comparison table showing all SLOs and their current status.
    Useful when viewing with 'All' selected in template variables.
  |||,
  fieldConfig: {
    defaults: {
      custom: {
        displayMode: 'color-background-solid',
      },
      thresholds: {
        mode: 'absolute',
        steps: [
          { color: 'red', value: 0 },
          { color: 'yellow', value: 95 },
          { color: 'green', value: 99 },
        ],
      },
      unit: 'percent',
      decimals: 3,
    },
  },
  gridPos: { h: 8, w: 24, x: 0, y: 26 },
  id: 9,
  options: {
    showHeader: true,
    sortBy: [{ displayName: 'SLI Value', desc: true }],
  },
  targets: [{
    expr: 'osko_sli_measurement{slo_name=~"$slo_name", service=~"$service", window=~"$window"} * 100',
    format: 'table',
    instant: true,
  }],
  title: 'SLO Comparison',
  type: 'table',
  transformations: [
    {
      id: 'organize',
      options: {
        excludeByName: {
          'Time': true,
          'Value': false,
        },
        indexByName: {
          'slo_name': 0,
          'service': 1,
          'window': 2,
          'Value': 3,
        },
        renameByName: {
          'slo_name': 'SLO Name',
          'service': 'Service',
          'window': 'Window',
          'Value': 'SLI Value (%)',
        },
      },
    },
  ],
};

// Dashboard definition with improved settings
dashboard.new(
  title='SLO Performance Dashboard (Improved)',
  uid='slo-performance-improved',
  tags=['slo', 'sre', 'osko', 'improved', 'performance'],
  time_from='now-30d',
  time_to='now',
  refresh='30s',
  editable=true,
  graphTooltip='shared_crosshair',
)
.addTemplates(templates)
.addPanels([
  // Row 1: Status Overview
  statusRow,
  compliancePanel,
  sliStatusPanel,
  sloTargetPanel,
  errorBudgetPanel,
  
  // Row 2: Trends
  trendsRow,
  sliTrendPanel,
  burndownPanel,
  
  // Row 3: Burn Rate
  burnRateRow,
  burnRatePanel,
  eventsSummaryPanel,
  
  // Row 4: Comparison (collapsed)
  comparisonRow,
  sloComparisonPanel,
]) + {
  // Annotations for alerts
  annotations: {
    list: [
      {
        datasource: { type: 'prometheus', uid: '$datasource' },
        enable: true,
        iconColor: 'red',
        name: 'SLO Alerts',
        expr: 'ALERTS{alertname=~"SLOBudgetBurn.*", slo_name=~"$slo_name", service=~"$service"}',
        tagKeys: 'alertname,severity',
        textFormat: '{{ alertname }} - {{ severity }}',
        titleFormat: 'SLO Alert',
      },
    ],
  },
  // Time picker options
  timepicker: {
    refresh_intervals: ['5s', '10s', '30s', '1m', '5m', '15m', '30m', '1h', '2h', '1d'],
    time_options: ['5m', '15m', '1h', '6h', '12h', '24h', '2d', '7d', '30d', '90d', '6M', '1y'],
  },
  // Templating list
  templating: {
    list: templates,
  },
  // Links to other dashboards
  links: [
    {
      title: 'OSKO Dashboards',
      type: 'dashboards',
      asDropdown: true,
      includeVars: true,
      keepTime: true,
      tags: ['osko'],
    },
  ],
  fiscalYearStartMonth: 0,
  preload: false,
  schemaVersion: 41,
  version: 1,
}
