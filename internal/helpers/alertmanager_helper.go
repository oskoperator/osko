package helpers

import (
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateAlertManagerConfig(alertNotificationTarget *openslov1.AlertNotificationTarget) (*monitoringv1alpha1.AlertmanagerConfig, error) {
	typeMeta := metav1.TypeMeta{
		Kind:       "AlertmanagerConfig",
		APIVersion: "monitoring.coreos.com/v1alpha1",
	}
	objectMeta := metav1.ObjectMeta{
		Name:      alertNotificationTarget.Name,
		Namespace: alertNotificationTarget.Namespace,
	}

	opsgenieConfig := monitoringv1alpha1.OpsGenieConfig{
		APIURL:   alertNotificationTarget.Spec.Target.OpsGenie.APIURL,
		APIKey:   alertNotificationTarget.Spec.Target.OpsGenie.APIKey,
		Priority: alertNotificationTarget.Spec.Target.OpsGenie.Priority,
	}

	receiver := monitoringv1alpha1.Receiver{
		Name: "OpeGenie",
		OpsGenieConfigs: []monitoringv1alpha1.OpsGenieConfig{
			opsgenieConfig,
		},
	}

	alertManagerConfigSpec := monitoringv1alpha1.AlertmanagerConfigSpec{
		Receivers: []monitoringv1alpha1.Receiver{
			receiver,
		},
	}

	alertManagerConfig := &monitoringv1alpha1.AlertmanagerConfig{
		TypeMeta:   typeMeta,
		ObjectMeta: objectMeta,
		Spec:       alertManagerConfigSpec,
	}

	return alertManagerConfig, nil
}
