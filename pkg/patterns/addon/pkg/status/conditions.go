package status

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

const (
	AbnormalReason = "AbnormalTrueConditions"
	NormalReason   = "Normal"
	ReadyType      = "Ready"
)

// SetInProgress set the present condition to a single condition with type "Ready" and status "false". This means
// the current resources is still reconciling. If any deployment manifests are abnormal, their abnormal status condition will
// be recorded in the `message` field.
func SetInProgress(commonStatus *addonsv1alpha1.CommonStatus, abnormalTrueConditions []status.Condition) {
	setCondition(metav1.ConditionFalse, commonStatus, abnormalTrueConditions)
}

// SetReady set the present condition to a single condition with type "Ready" and status "true". This means
// all the deployment manifests are reconciled.
func SetReady(commonStatus *addonsv1alpha1.CommonStatus, abnormalTrueConditions []status.Condition) {
	setCondition(metav1.ConditionTrue, commonStatus, abnormalTrueConditions)
}

func setCondition(status metav1.ConditionStatus, commonStatus *addonsv1alpha1.CommonStatus, abnormalTrueConditions []status.Condition) {
	reason, message := humanMessagefromConditions(abnormalTrueConditions)
	commonStatus.Conditions = []*metav1.Condition{{
		Status:             status,
		Type:               ReadyType,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	},
	}
}

// humanMessagefromConditions summarize the kstatus abnormal-true conditions to a reason with human-readable message.
// The "reason" should be "Normal" if no deployment manifests have abnormal conditions, or "ContainAbnormalTrueConditions"
// as long as one deployment manifest has an abnormal condition.
// THe "message" contains the each abnormal condition's "reason" and "message".
// e.g.
//
//	 conditions:
//	 - reason: ContainAbnormalTrueConditions
//		  message: |-
//		    apps/v1, Kind=Deployment/argocd/argocd-repo-server:Deployment does not have minimum availability.
//		    apps/v1, Kind=Deployment/argocd/argocd-server:Deployment does not have minimum availability.
//		  reason: ContainAbnormalTrueConditions
func humanMessagefromConditions(conditions []status.Condition) (reason, message string) {
	if len(conditions) == 0 {
		return NormalReason, "all manifests are reconciled."
	}
	abnormalMessage := ""
	for _, cond := range conditions {
		if cond.Message != "" {
			abnormalMessage += fmt.Sprintln(cond.Message)
		}
	}
	return AbnormalReason, abnormalMessage
}
