package controller

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
)

// The Thanos samples are what users copy to adopt the backend. Applying them
// against the real generated CRDs catches a misspelled field or an out-of-enum
// value that no other test would see.
var _ = Describe("Thanos sample manifests", func() {
	decodeSample := func(name string, obj client.Object) {
		f, err := os.Open(filepath.Join("..", "..", "..", "config", "samples", name))
		Expect(err).NotTo(HaveOccurred(), "sample file must exist")
		defer f.Close()

		Expect(yaml.NewYAMLOrJSONDecoder(f, 4096).Decode(obj)).To(Succeed(),
			"sample must decode into its typed object")
		obj.SetNamespace("default")
	}

	It("applies the Thanos datasource sample", func() {
		ds := &openslov1.Datasource{}
		decodeSample("openslo_v1_datasource_thanos.yaml", ds)

		Expect(ds.Spec.Type).To(Equal("thanos"),
			"the sample must actually exercise the thanos path")

		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("applies the Thanos SLO sample", func() {
		slo := &openslov1.SLO{}
		decodeSample("openslo_v1_slo_thanos.yaml", slo)

		Expect(slo.ObjectMeta.Annotations).To(HaveKeyWithValue("osko.dev/datasourceRef", "thanos-ds"),
			"the SLO sample must point at the datasource sample")

		Expect(k8sClient.Create(context.Background(), slo)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), slo)).To(Succeed())
	})
})
