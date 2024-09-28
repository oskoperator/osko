package helpers

import (
	"encoding/json"
	realopenslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
)

func GetQuery(input realopenslov1.SLIMetricSource) string {
	var output oskov1alpha1.MetricSourceSpec
	json.Unmarshal(input.MetricSourceSpec, &output)
	return output.Query
}

func ConstructConnectionDetails(ds *openslov1.Datasource) (details oskov1alpha1.ConnectionDetails) {
	json.Unmarshal(ds.Spec.ConnectionDetails, &details)
	return details
}
