package helpers

import (
	openslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
)

func GetQuery(input openslov1.SLIMetricSource) string {
	return input.MetricSourceSpec["query"].(string)
}
