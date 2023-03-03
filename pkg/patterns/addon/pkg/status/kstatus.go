package status

import (
	"context"

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
		currentStatus.Healthy = false
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
		}

		aggregatedPhase := string(aggregateStatus(statusMap))

		if currentStatus.Phase != aggregatedPhase {
			currentStatus.Phase = aggregatedPhase
		}
	}

	if err := utils.SetCommonStatus(info.Subject, currentStatus); err != nil {
		return err
	}

	return nil
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
