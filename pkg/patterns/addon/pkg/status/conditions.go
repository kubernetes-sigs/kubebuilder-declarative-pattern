package status

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

const (
	AbnormalReason = "ManifestsNotReady"
	NormalReason   = "Normal"
	ReadyType      = "Ready"
)

// SetInProgress set the present condition to a single condition with type "Ready" and status "false". This means
// the current resources is still reconciling. If any deployment manifests are abnormal, their abnormal status condition will
// be recorded in the `message` field.
func SetInProgress(conditions *[]metav1.Condition) {
	newCondition := metav1.Condition{}
	newCondition.Status = metav1.ConditionFalse
	newCondition.Type = ReadyType

	meta.SetStatusCondition(conditions, newCondition)
}

// SetReady set the present condition to a single condition with type "Ready" and status "true". This means
// all the deployment manifests are reconciled.
func SetReady(conditions *[]metav1.Condition) {
	newCondition := metav1.Condition{}
	newCondition.Status = metav1.ConditionTrue
	newCondition.Type = ReadyType

	meta.SetStatusCondition(conditions, newCondition)
}

// new returns a Condition object with human-readable message and reason.
// The "reason" should be "Normal" if no deployment manifests have abnormal conditions, or "ManifestsNotReady"
// as long as one deployment manifest has an abnormal condition.
// The "message" contains each abnormal condition's "reason" and "message".
// e.g.
//
//	 conditions:
//	 - reason: ManifestsNotReady
//	   message: |-
//		    apps/v1, Kind=Deployment/argocd/argocd-repo-server:Deployment does not have minimum availability.
//		    apps/v1, Kind=Deployment/argocd/argocd-server:Deployment does not have minimum availability.
func new(conditions []status.Condition) metav1.Condition {
	if len(conditions) == 0 {
		return metav1.Condition{
			Reason:  NormalReason,
			Message: "all manifests are reconciled.",
		}
	}
	abnormalMessage := ""
	for _, cond := range conditions {
		if cond.Message != "" {
			abnormalMessage += cond.Message + "\n"
		}
	}

	return metav1.Condition{
		Reason:  AbnormalReason,
		Message: strings.TrimSuffix(abnormalMessage, "\n"),
	}
}
