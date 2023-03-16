package status

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/utils"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
)

type kstatusAggregator struct {
}

// TODO: Create a version that doesn't need reconciler or client?
func NewKstatusAgregator(_ client.Client, _ *declarative.Reconciler) *kstatusAggregator {
	return &kstatusAggregator{}
}

func (k *kstatusAggregator) BuildStatus(ctx context.Context, info *declarative.StatusInfo) error {
	log := log.FromContext(ctx)

	currentStatus, err := utils.GetCommonStatus(info.Subject)
	if err != nil {
		log.Error(err, "error retrieving status")
		return err
	}

	shouldComputeHealthFromObjects := info.Manifest != nil && info.LiveObjects != nil
	if info.Err != nil {
		switch info.KnownError {
		case declarative.KnownErrorApplyFailed:
			currentStatus.Phase = "Applying"
			// computeHealthFromObjects if we can (leave unchanged)
		case declarative.KnownErrorVersionCheckFailed:
			currentStatus.Phase = "VersionMismatch"
			shouldComputeHealthFromObjects = false
		default:
			currentStatus.Phase = "InternalError"
			shouldComputeHealthFromObjects = false
		}
	}

	// Here we augment each deployment manifests abnormal conditions and determine the declarativeObject's present
	// condition from these conditions.
	// https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus#conditions
	var abnormalConditions []status.Condition

	if shouldComputeHealthFromObjects {
		statusMap := make(map[status.Status]bool)
		for _, object := range info.Manifest.Items {
			gvk := object.GroupVersionKind()
			nn := object.NamespacedName()

			log := log.WithValues("kind", gvk.Kind).WithValues("name", nn.Name).WithValues("namespace", nn.Namespace)

			unstruct, err := info.LiveObjects(ctx, gvk, nn)
			if err != nil {
				log.Error(err, "unable to get object to determine status")
				statusMap[status.UnknownStatus] = true
				continue
			}
			res, err := status.Compute(unstruct)
			if err != nil {
				log.Error(err, "error getting status of resource")
				statusMap[status.UnknownStatus] = true
			} else if res != nil {
				log.WithValues("status", res.Status).WithValues("message", res.Message).Info("Got status of resource:")
				statusMap[res.Status] = true
			} else {
				log.Info("resource status was nil")
				statusMap[status.UnknownStatus] = true
			}
			conds := getAbnormalConditions(ctx, unstruct)
			abnormalConditions = append(abnormalConditions, conds...)
		}

		// Summarize all the deployment manifests statuses to a single results.
		aggregatedPhase := aggregateStatus(statusMap)
		// Update the Conditions for the declarativeObject status.
		if aggregatedPhase == status.CurrentStatus {
			SetReady(&currentStatus, abnormalConditions)
		} else {
			SetInProgress(&currentStatus, abnormalConditions)
		}
		currentStatus.Phase = string(aggregatedPhase)
	}
	currentStatus.Healthy = currentStatus.Phase == string(status.CurrentStatus)
	if err := utils.SetCommonStatus(info.Subject, currentStatus); err != nil {
		return err
	}
	return nil
}

// getAbnormalConditions calculates the abnormal-true conditions that best describe the current state of the deployment manifests.
func getAbnormalConditions(ctx context.Context, unstruct *unstructured.Unstructured) []status.Condition {
	log := log.FromContext(ctx).WithValues("object", unstruct)

	// Normalize the deployment manifest conditions.
	// The augmented condition "type" should only be "Stalled" or "Reconciling".
	if err := status.Augment(unstruct); err != nil {
		log.Error(err, "unable to augment conditions")
		return nil
	}

	// The default unstructured.Unstructured does not have a structured "status.conditions" fields.
	obj, err := status.GetObjectWithConditions(unstruct.Object)
	if err != nil {
		log.Error(err, "unable to get conditions")
		return nil
	}

	if len(obj.Status.Conditions) == 0 {
		return nil
	}

	// kstatus appends the augmented conditions to the end.
	cond := obj.Status.Conditions[len(obj.Status.Conditions)-1]
	if cond.Type == string(status.ConditionStalled) || cond.Type == string(status.ConditionReconciling) {
		return []status.Condition{{
			Type:    status.ConditionType(cond.Type),
			Status:  cond.Status,
			Reason:  cond.Reason,
			Message: getGVKNN(unstruct) + ":" + cond.Message,
		}}
	}
	return nil
}

func getGVKNN(obj *unstructured.Unstructured) string {
	return obj.GroupVersionKind().String() + "/" + obj.GetNamespace() + "/" + obj.GetName()
}

func aggregateStatus(m map[status.Status]bool) status.Status {
	inProgress := m[status.InProgressStatus]
	terminating := m[status.TerminatingStatus]

	failed := m[status.FailedStatus]

	if inProgress || terminating {
		return status.InProgressStatus
	}

	if failed {
		return status.FailedStatus
	}

	return status.CurrentStatus
}
