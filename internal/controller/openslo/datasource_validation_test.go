package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
)

var _ = Describe("Datasource spec.type validation", func() {
	newDatasource := func(name, dsType string) *openslov1.Datasource {
		return &openslov1.Datasource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: openslov1.DatasourceSpec{
				Type: dsType,
				ConnectionDetails: oskov1alpha1.ConnectionDetails{
					Address: "http://example:9090",
				},
			},
		}
	}

	It("rejects a misspelled datasource type", func() {
		err := k8sClient.Create(context.Background(), newDatasource("bad-type", "thanso"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.type"))
		Expect(err.Error()).To(ContainSubstring("Unsupported value"))
	})

	It("accepts thanos", func() {
		ds := newDatasource("good-thanos", "thanos")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("accepts prometheus", func() {
		ds := newDatasource("good-prometheus", "prometheus")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("accepts mimir", func() {
		ds := newDatasource("good-mimir", "mimir")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("accepts cortex", func() {
		ds := newDatasource("good-cortex", "cortex")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("accepts victoriametrics", func() {
		ds := newDatasource("good-vm", "victoriametrics")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	// Load-bearing, not cosmetic: an enum does not validate an absent field, so
	// without the default an upgraded type-less Datasource fails backend.Parse
	// above PrometheusRule creation and stops producing rules at all. Spec D5.
	It("defaults an omitted type to mimir", func() {
		ctx := context.Background()
		ds := newDatasource("default-type", "")
		Expect(ds.Spec.Type).To(BeEmpty(), "the object must be sent without a type for the default to apply")
		Expect(k8sClient.Create(ctx, ds)).To(Succeed())

		read := &openslov1.Datasource{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "default-type", Namespace: "default"}, read)).To(Succeed())
		Expect(read.Spec.Type).To(Equal("mimir"),
			"an upgraded Datasource with no spec.type must keep working, not fail backend.Parse")

		Expect(k8sClient.Delete(ctx, read)).To(Succeed())
	})
})
