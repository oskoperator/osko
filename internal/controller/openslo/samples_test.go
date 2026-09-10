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

// The Thanos samples are what users copy to adopt the backend. Decoding them
// strictly catches a misspelled field, and applying them against the real
// generated CRDs catches an out-of-enum value. Strictness is load-bearing:
// a non-strict decoder drops an unknown field client-side, so the API server
// never sees it and the spec would stay green while `kubectl apply` — which
// has validated fields strictly since 1.25 — rejects the sample.
var _ = Describe("Thanos sample manifests", func() {
	decodeSample := func(name string, obj client.Object) {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "samples", name))
		Expect(err).NotTo(HaveOccurred(), "sample file must exist")

		Expect(yaml.UnmarshalStrict(data, obj)).To(Succeed(),
			"sample must decode into its typed object with no unknown fields")
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
