package status

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

const (
	AbnormalReason = "ManifestsNotReady"
	NormalReason   = "Normal"
	ReadyType      = "Ready"
)

// // SetInProgress set the present condition to a single condition with type "Ready" and status "false". This means
// // the current resources is still reconciling. If any deployment manifests are abnormal, their abnormal status condition will
// // be recorded in the `message` field.
// func SetInProgress(conditions *[]metav1.Condition, message string) {
// 	newCondition := metav1.Condition{}
// 	newCondition.Status = metav1.ConditionFalse
// 	newCondition.Type = ReadyType

// 	meta.SetStatusCondition(conditions, newCondition)
// }

// // SetReady set the present condition to a single condition with type "Ready" and status "true". This means
// // all the deployment manifests are reconciled.
// func SetReady(conditions *[]metav1.Condition) {
// 	newCondition := metav1.Condition{}
// 	newCondition.Status = metav1.ConditionTrue
// 	newCondition.Type = ReadyType

// 	meta.SetStatusCondition(conditions, newCondition)
// }

// buildReadyCondition returns a Condition object with human-readable message and reason.
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
func buildReadyCondition(isReady bool, abnormalConditions []status.Condition) metav1.Condition {
	var readyCondition metav1.Condition
	readyCondition.Type = ReadyType

	if isReady {
		readyCondition.Status = metav1.ConditionTrue
	} else {
		readyCondition.Status = metav1.ConditionFalse
	}

	if len(abnormalConditions) == 0 {
		readyCondition.Reason = NormalReason
		readyCondition.Message = "all manifests are reconciled."
	} else {
		var messages []string
		for _, cond := range abnormalConditions {
			if cond.Message != "" {
				messages = append(messages, cond.Message)
			}
		}

		readyCondition.Reason = AbnormalReason

		readyCondition.Message = strings.Join(messages, "\n")
	}

	return readyCondition
}
