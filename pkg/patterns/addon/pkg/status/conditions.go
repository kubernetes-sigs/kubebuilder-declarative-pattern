package status

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

const (
	AbnormalReason = "ContainAbnormalTrueConditions"
	NormalReason   = "Normal"
	ReadyType      = "Ready"
)

// GetCondition returns the first met Condition whose type matches the given condition type.
func GetCondition(conditions []*metav1.Condition, condType status.ConditionType) *metav1.Condition {
	for i, condition := range conditions {
		if condition.Type == string(condType) {
			return conditions[i]
		}
	}
	return nil
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

// setCondition sets a Condition.
func setCondition(commonStatus *addonsv1alpha1.CommonStatus, condType status.ConditionType, condStatus metav1.ConditionStatus,
	abnormalTrueConditions []status.Condition) bool {
	// If no desired ConditionType, append the new Condition at the end.
	condition := GetCondition(commonStatus.Conditions, condType)
	if condition == nil {
		i := len(commonStatus.Conditions)
		commonStatus.Conditions = append(commonStatus.Conditions, &metav1.Condition{
			Type:               string(condType),
			LastTransitionTime: metav1.Now(),
		})
		condition = commonStatus.Conditions[i]
	}
	reason, message := humanMessagefromConditions(abnormalTrueConditions)
	var transitioned bool
	if condition.Status != condStatus {
		condition.Status = condStatus
		transitioned = true
	} else if condition.Reason == reason && condition.Message == message {
		return false
	}
	condition.Reason = reason
	condition.Message = message
	return transitioned
}

// RemoveCondition removes all the matched-type Conditions.
func RemoveCondition(commonStatus *addonsv1alpha1.CommonStatus, condType status.ConditionType) {
	var newConditions []*metav1.Condition
	for _, c := range commonStatus.Conditions {
		if status.ConditionType(c.Type) == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	commonStatus.Conditions = newConditions
}

// SetReconciling set the condition to "Reconciled"
func SetReconciling(commonStatus *addonsv1alpha1.CommonStatus, abnormalTrueConditions []status.Condition) {
	transitioned := setCondition(commonStatus, status.ConditionReconciling, metav1.ConditionTrue, abnormalTrueConditions)
	if transitioned {
		RemoveCondition(commonStatus, status.ConditionStalled)
	}
}

// SetStalled set the condition to "Stalled" with manifest abnormal messages.
func SetStalled(commonStatus *addonsv1alpha1.CommonStatus, abnormalTrueConditions []status.Condition) {
	transitioned := setCondition(commonStatus, status.ConditionStalled, metav1.ConditionTrue, abnormalTrueConditions)
	if transitioned {
		RemoveCondition(commonStatus, status.ConditionReconciling)
	}
}

// SetNormal guarantees the status.conditions has a "Success" condition.
func SetNormal(commonStatus *addonsv1alpha1.CommonStatus) {
	for _, cond := range commonStatus.Conditions {
		if cond.Type == ReadyType {
			return
		}
	}
	commonStatus.Conditions = append(commonStatus.Conditions, &metav1.Condition{
		Status: metav1.ConditionTrue,
		// "Ready" with true is considered by kstatus as the resource is fully reconciled.
		Type:               ReadyType,
		LastTransitionTime: metav1.Now(),
		Reason:             NormalReason,
		Message:            "all manifests are reconciled",
	})
}
