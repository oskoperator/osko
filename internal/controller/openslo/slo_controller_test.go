package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

var _ = Describe("SLO Controller Ownership Model", func() {
	Context("When reconciling SLO with inline SLI", func() {
		const resourceName = "test-slo-inline"
		const namespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: namespace,
		}

		var datasource *openslov1.Datasource
		var slo *openslov1.SLO

		BeforeEach(func() {
			By("creating a Datasource")
			datasource = &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-datasource",
					Namespace: namespace,
				},
				Spec: openslov1.DatasourceSpec{
					Type: "mimir",
					ConnectionDetails: oskov1alpha1.ConnectionDetails{
						Address:       "http://localhost:9009/",
						SourceTenants: []string{"test"},
						TargetTenant:  "test",
					},
				},
			}
			Expect(k8sClient.Create(ctx, datasource)).To(Succeed())

			By("creating SLO with inline SLI")
			slo = &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
					Annotations: map[string]string{
						"osko.dev/datasourceRef": "test-datasource",
					},
				},
				Spec: openslov1.SLOSpec{
					Description:     "Test SLO with inline SLI",
					Service:         "test-service",
					BudgetingMethod: "Occurrences",
					Indicator: &openslov1.Indicator{
						Metadata: metav1.ObjectMeta{
							Name: "test-inline-sli",
						},
						Spec: openslov1.SLISpec{
							Description: "Test inline SLI",
							RatioMetric: openslov1.RatioMetricSpec{
								Good: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-datasource",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(good_requests)",
										},
									},
								},
								Total: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-datasource",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(total_requests)",
										},
									},
								},
							},
						},
					},
					Objectives: []openslov1.ObjectivesSpec{
						{
							Target: "0.99",
						},
					},
					TimeWindow: []openslov1.TimeWindowSpec{
						{
							Duration:  openslov1.Duration("28d"),
							IsRolling: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, slo)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup SLO")
			if slo != nil {
				Expect(k8sClient.Delete(ctx, slo)).To(Succeed())
			}
			By("Cleanup Datasource")
			if datasource != nil {
				Expect(k8sClient.Delete(ctx, datasource)).To(Succeed())
			}
		})

		It("should create and own inline SLI", func() {
			By("Reconciling the SLO")
			controllerReconciler := &SLOReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying inline SLI was created")
			inlineSLI := &openslov1.SLI{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-inline-sli",
				Namespace: namespace,
			}, inlineSLI)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying SLI has correct owner reference")
			Expect(inlineSLI.OwnerReferences).To(HaveLen(1))
			Expect(inlineSLI.OwnerReferences[0].Name).To(Equal(resourceName))
			Expect(inlineSLI.OwnerReferences[0].Kind).To(Equal("SLO"))

			By("Verifying SLO has finalizer")
			updatedSLO := &openslov1.SLO{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedSLO)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSLO.Finalizers).To(ContainElement("finalizer.slo.osko.dev"))
		})

		It("should create and own PrometheusRule", func() {
			By("Reconciling the SLO")
			controllerReconciler := &SLOReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying PrometheusRule was created")
			prometheusRule := &monitoringv1.PrometheusRule{}
			err = k8sClient.Get(ctx, typeNamespacedName, prometheusRule)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying PrometheusRule has correct owner reference")
			Expect(prometheusRule.OwnerReferences).To(HaveLen(1))
			Expect(prometheusRule.OwnerReferences[0].Name).To(Equal(resourceName))
			Expect(prometheusRule.OwnerReferences[0].Kind).To(Equal("SLO"))
		})
	})

	Context("When reconciling SLO with SLI reference", func() {
		const sloName = "test-slo-ref"
		const namespace = "default"

		ctx := context.Background()

		var sliName string

		sloNamespacedName := types.NamespacedName{
			Name:      sloName,
			Namespace: namespace,
		}

		var datasource *openslov1.Datasource
		var sharedSLI *openslov1.SLI
		var slo *openslov1.SLO

		BeforeEach(func() {
			sliName = "test-shared-sli"

			By("creating a Datasource")
			datasource = &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-datasource-ref",
					Namespace: namespace,
				},
				Spec: openslov1.DatasourceSpec{
					Type: "mimir",
					ConnectionDetails: oskov1alpha1.ConnectionDetails{
						Address:       "http://localhost:9009/",
						SourceTenants: []string{"test"},
						TargetTenant:  "test",
					},
				},
			}
			Expect(k8sClient.Create(ctx, datasource)).To(Succeed())

			By("creating shared SLI")
			sharedSLI = &openslov1.SLI{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliName,
					Namespace: namespace,
				},
				Spec: openslov1.SLISpec{
					Description: "Shared SLI for testing",
					RatioMetric: openslov1.RatioMetricSpec{
						Good: openslov1.MetricSpec{
							MetricSource: openslov1.MetricSource{
								MetricSourceRef: "test-datasource-ref",
								Type:            "Mimir",
								Spec: openslov1.MetricSourceSpec{
									Query: "sum(good_requests)",
								},
							},
						},
						Total: openslov1.MetricSpec{
							MetricSource: openslov1.MetricSource{
								MetricSourceRef: "test-datasource-ref",
								Type:            "Mimir",
								Spec: openslov1.MetricSourceSpec{
									Query: "sum(total_requests)",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sharedSLI)).To(Succeed())

			By("creating SLO with SLI reference")
			slo = &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sloName,
					Namespace: namespace,
					Annotations: map[string]string{
						"osko.dev/datasourceRef": "test-datasource-ref",
					},
				},
				Spec: openslov1.SLOSpec{
					Description:     "Test SLO with SLI reference",
					Service:         "test-service",
					BudgetingMethod: "Occurrences",
					IndicatorRef:    &sliName,
					Objectives: []openslov1.ObjectivesSpec{
						{
							Target: "0.99",
						},
					},
					TimeWindow: []openslov1.TimeWindowSpec{
						{
							Duration:  openslov1.Duration("28d"),
							IsRolling: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, slo)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup SLO")
			if slo != nil {
				Expect(k8sClient.Delete(ctx, slo)).To(Succeed())
			}
			By("Cleanup shared SLI")
			if sharedSLI != nil {
				Expect(k8sClient.Delete(ctx, sharedSLI)).To(Succeed())
			}
			By("Cleanup Datasource")
			if datasource != nil {
				Expect(k8sClient.Delete(ctx, datasource)).To(Succeed())
			}
		})

		It("should reference but not own shared SLI", func() {
			By("Reconciling the SLO")
			controllerReconciler := &SLOReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: sloNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying shared SLI still exists and is not owned by SLO")
			retrievedSLI := &openslov1.SLI{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      sliName,
				Namespace: namespace,
			}, retrievedSLI)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying SLI does not have SLO as owner")
			for _, ownerRef := range retrievedSLI.OwnerReferences {
				Expect(ownerRef.Name).NotTo(Equal(sloName))
			}
		})
	})

	Context("When reconciling SLO with magic alerting", func() {
		const resourceName = "test-slo-alerting"
		const namespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: namespace,
		}

		var datasource *openslov1.Datasource
		var slo *openslov1.SLO

		BeforeEach(func() {
			By("creating a Datasource")
			datasource = &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-datasource-alerting",
					Namespace: namespace,
				},
				Spec: openslov1.DatasourceSpec{
					Type: "mimir",
					ConnectionDetails: oskov1alpha1.ConnectionDetails{
						Address:       "http://localhost:9009/",
						SourceTenants: []string{"test"},
						TargetTenant:  "test",
					},
				},
			}
			Expect(k8sClient.Create(ctx, datasource)).To(Succeed())

			By("creating SLO with magic alerting enabled")
			slo = &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
					Annotations: map[string]string{
						"osko.dev/datasourceRef": "test-datasource-alerting",
						"osko.dev/magicAlerting": "true",
					},
				},
				Spec: openslov1.SLOSpec{
					Description:     "Test SLO with magic alerting",
					Service:         "test-service",
					BudgetingMethod: "Occurrences",
					Indicator: &openslov1.Indicator{
						Metadata: metav1.ObjectMeta{
							Name: "test-alerting-sli",
						},
						Spec: openslov1.SLISpec{
							Description: "Test SLI for alerting",
							RatioMetric: openslov1.RatioMetricSpec{
								Good: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-datasource-alerting",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(good_requests)",
										},
									},
								},
								Total: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-datasource-alerting",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(total_requests)",
										},
									},
								},
							},
						},
					},
					Objectives: []openslov1.ObjectivesSpec{
						{
							Target: "0.99",
						},
					},
					TimeWindow: []openslov1.TimeWindowSpec{
						{
							Duration:  openslov1.Duration("28d"),
							IsRolling: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, slo)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup SLO")
			if slo != nil {
				Expect(k8sClient.Delete(ctx, slo)).To(Succeed())
			}
			By("Cleanup Datasource")
			if datasource != nil {
				Expect(k8sClient.Delete(ctx, datasource)).To(Succeed())
			}
		})

		It("should create and own AlertManagerConfig", func() {
			By("Reconciling the SLO")
			controllerReconciler := &SLOReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying AlertManagerConfig was created")
			alertManagerConfig := &oskov1alpha1.AlertManagerConfig{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-alerting", resourceName),
				Namespace: namespace,
			}, alertManagerConfig)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying AlertManagerConfig has correct owner reference")
			Expect(alertManagerConfig.OwnerReferences).To(HaveLen(1))
			Expect(alertManagerConfig.OwnerReferences[0].Name).To(Equal(resourceName))
			Expect(alertManagerConfig.OwnerReferences[0].Kind).To(Equal("SLO"))

			By("Verifying AlertManagerConfig has correct labels and annotations")
			Expect(alertManagerConfig.Labels["osko.dev/slo"]).To(Equal(resourceName))
			Expect(alertManagerConfig.Annotations["osko.dev/datasourceRef"]).To(Equal("test-datasource-alerting"))
		})
	})

	Context("When testing cascading deletion", func() {
		const resourceName = "test-slo-cascade"
		const namespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: namespace,
		}

		var datasource *openslov1.Datasource

		BeforeEach(func() {
			By("creating a Datasource")
			datasource = &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-datasource-cascade",
					Namespace: namespace,
				},
				Spec: openslov1.DatasourceSpec{
					Type: "mimir",
					ConnectionDetails: oskov1alpha1.ConnectionDetails{
						Address:       "http://localhost:9009/",
						SourceTenants: []string{"test"},
						TargetTenant:  "test",
					},
				},
			}
			Expect(k8sClient.Create(ctx, datasource)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup Datasource")
			if datasource != nil {
				Expect(k8sClient.Delete(ctx, datasource)).To(Succeed())
			}
		})

		It("should cleanup owned resources when SLO is deleted", func() {
			By("Creating SLO with inline SLI and magic alerting")
			slo := &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
					Annotations: map[string]string{
						"osko.dev/datasourceRef": "test-datasource-cascade",
						"osko.dev/magicAlerting": "true",
					},
				},
				Spec: openslov1.SLOSpec{
					Description:     "Test SLO for cascade deletion",
					Service:         "test-service",
					BudgetingMethod: "Occurrences",
					Indicator: &openslov1.Indicator{
						Metadata: metav1.ObjectMeta{
							Name: "test-cascade-sli",
						},
						Spec: openslov1.SLISpec{
							Description: "Test SLI for cascade deletion",
							RatioMetric: openslov1.RatioMetricSpec{
								Good: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-datasource-cascade",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(good_requests)",
										},
									},
								},
								Total: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-datasource-cascade",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(total_requests)",
										},
									},
								},
							},
						},
					},
					Objectives: []openslov1.ObjectivesSpec{
						{
							Target: "0.99",
						},
					},
					TimeWindow: []openslov1.TimeWindowSpec{
						{
							Duration:  openslov1.Duration("28d"),
							IsRolling: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, slo)).To(Succeed())

			By("Reconciling the SLO to create owned resources")
			controllerReconciler := &SLOReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying owned resources were created")
			inlineSLI := &openslov1.SLI{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-cascade-sli",
				Namespace: namespace,
			}, inlineSLI)
			Expect(err).NotTo(HaveOccurred())

			alertManagerConfig := &oskov1alpha1.AlertManagerConfig{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-alerting", resourceName),
				Namespace: namespace,
			}, alertManagerConfig)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the SLO")
			Expect(k8sClient.Delete(ctx, slo)).To(Succeed())

			By("Reconciling deletion")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying owned resources are eventually deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-cascade-sli",
					Namespace: namespace,
				}, inlineSLI)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      fmt.Sprintf("%s-alerting", resourceName),
					Namespace: namespace,
				}, alertManagerConfig)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})
	})
})
