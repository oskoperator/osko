package helpers

import (
	"encoding/json"
	realopenslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	"reflect"
)

func GetQuery(input realopenslov1.SLIMetricSource) string {
	// return input.MetricSourceSpec["query"].(string)
	// return string(input.MetricSourceSpec[])
	// TODO: fix this

	// test := []byte{}
	// for _, v := range input.MetricSourceSpec {
	// 	v.UnmarshalJSON(test)
	// }
	return "hello"
}

func ConstructConnectionDetails(ds *openslov1.Datasource) (details oskov1alpha1.ConnectionDetails) {
	v := reflect.ValueOf(details).Elem()
	for key, value := range ds.Spec.ConnectionDetails {
		field := v.FieldByName(key)
		if !field.IsValid() {
			continue
		}

		if !field.CanSet() {
			continue
		}
		json.Unmarshal(value, &field)
	}

	return details
}
