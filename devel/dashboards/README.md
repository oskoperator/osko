# OSKO Grafana Dashboards

This directory contains Grafonnet templates for generating Grafana dashboards to monitor SLO performance and metrics.

## SLO Performance Dashboard

The `slo-performance-dashboard.jsonnet` template creates a dashboard matching the OSKO SLO monitoring requirements with the following panels:

### Panels Included:
- **STATUS**: Current SLI value as percentage with color-coded thresholds
- **ERROR BUDGET LEFT**: Remaining error budget as horizontal bar gauge with time remaining
- **Error budget burndown**: Time series chart showing error budget consumption over time
- **Burn rate**: Time series chart showing current burn rate spikes

### Template Variables:
- `$datasource`: Prometheus datasource selection
- `$slo_name`: SLO name selector
- `$service`: Service name selector

### Expected Metrics:
The dashboard expects the following Prometheus metrics to be available from OSKO:
- `osko_sli_measurement{slo_name, service, window}`: Current SLI measurement (0-1, displayed as percentage)
- `osko_error_budget_value{slo_name, service, window}`: Error budget consumed (0-1)
- `osko_slo_target{slo_name, service}`: SLO target threshold
- `osko_error_budget_burn_rate{slo_name, service, window}`: Rate of error budget consumption

### Calculated Metrics:
The dashboard calculates these derived metrics:
- **Error Budget Remaining**: `((sli_measurement - slo_target) / (1 - slo_target)) * 100` (as percentage)
- **Burn Rate**: Uses `osko_error_budget_burn_rate` metric directly
- **Time Left** (if needed): `error_budget_remaining / burn_rate` (in time units)

### Important Note:
`osko_error_budget_value` represents the current error rate (1 - sli_measurement), not error budget consumed. The dashboard correctly calculates error budget remaining relative to the SLO target.

## Usage

### Prerequisites
1. Install Grafonnet library:
   ```bash
   jb install  # Installs dependencies from jsonnetfile.lock.json to vendor/
   ```

2. Ensure you have `jsonnet` command available

**Note**: The `vendor/` directory is generated and not committed to git. Use `jb install` to regenerate it from the lock file.

### Generate Dashboard JSON
```bash
jsonnet slo-performance-dashboard.jsonnet > slo-performance-dashboard.json
```

### Import to Grafana
1. Open Grafana UI
2. Go to "+" → "Import"
3. Upload the generated JSON file or paste its contents
4. Configure the Prometheus datasource
5. Save the dashboard

### Example jsonnetfile.json
```json
{
  "version": 1,
  "dependencies": [
    {
      "source": {
        "git": {
          "remote": "https://github.com/grafana/grafonnet-lib.git",
          "subdir": "grafonnet"
        }
      },
      "version": "master"
    }
  ],
  "legacyImports": true
}
```

## Customization

The template can be customized by modifying:
- Metric names and labels to match your OSKO deployment
- Thresholds and colors for status indicators
- Time ranges and refresh intervals
- Panel layouts and sizing
- Additional template variables for filtering

## Integration with OSKO

This dashboard is designed to work with the OSKO operator's metric exposition. Ensure your OSKO deployment is configured to expose the required metrics through your Prometheus/Mimir setup.
