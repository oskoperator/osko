package v1

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

const counterProperty = "counter"

// RatioMetricSpec.Counter defaults to true via a CRD schema default, which only
// holds if two independent things stay true: the +kubebuilder:default marker is
// present, and the JSON tag has no omitempty. Break either and the default
// silently stops applying (or, with omitempty, flips an explicit false back to
// true when the SLO controller rewrites an inline indicator). Neither failure
// produces an error at runtime — just wrong SLI math — so both are asserted.

func TestRatioMetricCounterTagHasNoOmitEmpty(t *testing.T) {
	field, ok := reflect.TypeOf(RatioMetricSpec{}).FieldByName("Counter")
	if !ok {
		t.Fatal("RatioMetricSpec has no Counter field")
	}

	tag := field.Tag.Get("json")
	if strings.Contains(tag, "omitempty") {
		t.Errorf(
			"Counter json tag must not use omitempty (got %q): the SLO controller writes an SLI "+
				"from an inline indicator, and omitting the field lets the CRD default flip an "+
				"explicit false back to true",
			tag,
		)
	}
}

func TestGeneratedCRDsDefaultCounterToTrue(t *testing.T) {
	crds := []string{
		"openslo.com_slis.yaml",
		"openslo.com_slos.yaml",
	}

	for _, name := range crds {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "config", "crd", "bases", name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var doc map[string]interface{}
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("unmarshal %s: %v", path, err)
			}

			found := findCounterSchemas(doc)
			if len(found) == 0 {
				t.Fatalf("no %q property found in %s; regenerate with `make manifests`", counterProperty, name)
			}

			for i, schema := range found {
				if schema["default"] != true {
					t.Errorf(
						"%s: counter schema #%d has default %v, want true; is the "+
							"+kubebuilder:default=true marker still on RatioMetricSpec.Counter?",
						name, i, schema["default"],
					)
				}
			}
		})
	}
}

// findCounterSchemas walks an unmarshalled CRD and returns every schema sitting
// under a `properties.counter` key. RatioMetricSpec is embedded at several
// depths (SLI spec, SLO spec.indicator, SLO objectives[].indicator), so every
// occurrence has to be checked rather than just the first.
func findCounterSchemas(node interface{}) []map[string]interface{} {
	var found []map[string]interface{}

	switch typed := node.(type) {
	case map[string]interface{}:
		if properties, ok := typed["properties"].(map[string]interface{}); ok {
			if counter, ok := properties[counterProperty].(map[string]interface{}); ok {
				found = append(found, counter)
			}
		}
		for _, value := range typed {
			found = append(found, findCounterSchemas(value)...)
		}
	case []interface{}:
		for _, value := range typed {
			found = append(found, findCounterSchemas(value)...)
		}
	}

	return found
}
