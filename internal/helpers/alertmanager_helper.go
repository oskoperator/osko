package helpers

import (
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func CreateAlertManagerConfig(alertNotificationTarget *openslov1.AlertNotificationTarget) (*monitoringv1alpha1.AlertmanagerConfig, error) {
	// TODO: Add support for other notification targets as for now we only support OpsGenie and it is hardcoded just for testing purposes
	// TODO: Current OpenSLO specification only supports `string` as the type for target, so we would have to create a new CRD for each notification target
	var (
		receivers  []monitoringv1alpha1.Receiver
		routesJSON []apiextensionsv1.JSON
	)

	typeMeta := metav1.TypeMeta{
		Kind:       "AlertmanagerConfig",
		APIVersion: "monitoring.coreos.com/v1alpha1",
	}

	objectMeta := metav1.ObjectMeta{
		Name:      alertNotificationTarget.Name,
		Namespace: alertNotificationTarget.Namespace,
	}

	opsgenieConfig := monitoringv1alpha1.OpsGenieConfig{
		APIURL: alertNotificationTarget.Spec.Target.OpsGenie.APIURL,
		//APIKey:   alertNotificationTarget.Spec.Target.OpsGenie.APIKey,
		Priority: alertNotificationTarget.Spec.Target.OpsGenie.Priority,
	}

	opsGenieReceiver := monitoringv1alpha1.Receiver{
		Name: "OpsGenie",
		OpsGenieConfigs: []monitoringv1alpha1.OpsGenieConfig{
			opsgenieConfig,
		},
	}

	receivers = append(receivers, opsGenieReceiver)

	alertRoutes := []monitoringv1alpha1.Route{
		{
			Receiver: "heartbeat",
			Matchers: []monitoringv1alpha1.Matcher{
				{
					Name:  "alertname",
					Value: "DeadMansSwitch",
				},
			},
			GroupInterval:  "1m",
			RepeatInterval: "1m",
			Continue:       true,
		}, {
			Receiver: "heartbeat_betteruptime",
			Matchers: []monitoringv1alpha1.Matcher{
				{
					Name:  "alertname",
					Value: "DeadMansSwitch",
				},
			},
			GroupInterval:  "1m",
			RepeatInterval: "1m",
		}, {
			Receiver: "betteruptime",
			Matchers: []monitoringv1alpha1.Matcher{
				{
					Name:      "alertname",
					Value:     ".*",
					MatchType: monitoringv1alpha1.MatchRegexp,
				},
			},
			Continue: true,
		}, {
			Receiver: "opsgenie",
			Matchers: []monitoringv1alpha1.Matcher{
				{
					Name:      "alertname",
					Value:     ".*",
					MatchType: monitoringv1alpha1.MatchRegexp,
				},
			},
		},
	}

	for _, aroute := range alertRoutes {
		alertRoutesJSON, err := json.Marshal(aroute)
		if err != nil {
			return nil, err
		}

		route := apiextensionsv1.JSON{Raw: alertRoutesJSON}
		routesJSON = append(routesJSON, route)
	}

	routes := &monitoringv1alpha1.Route{
		Receiver: opsGenieReceiver.Name,
		GroupBy: []string{
			"alertname",
			"job",
			"statefulset",
			"daemonset",
		},
		Continue:       true,
		GroupWait:      "30s",
		GroupInterval:  "5min",
		RepeatInterval: "12h",
		Routes:         routesJSON,
	}

	alertManagerConfigSpec := monitoringv1alpha1.AlertmanagerConfigSpec{
		Route:     routes,
		Receivers: receivers,
	}

	alertManagerConfig := &monitoringv1alpha1.AlertmanagerConfig{
		TypeMeta:   typeMeta,
		ObjectMeta: objectMeta,
		Spec:       alertManagerConfigSpec,
	}

	return alertManagerConfig, nil
}
