package status

import (
	"context"
	"fmt"

	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/utils"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

type kstatusAggregator struct {
	client     client.Client
	reconciler *declarative.Reconciler
}

func NewKstatusAgregator(c client.Client, reconciler *declarative.Reconciler) *kstatusAggregator {
	return &kstatusAggregator{client: c, reconciler: reconciler}
}

func (k *kstatusAggregator) Reconciled(ctx context.Context, src declarative.DeclarativeObject,
	objs *manifest.Objects) error {
	log := log.Log

	statusMap := make(map[status.Status]bool)
	for _, object := range objs.Items {

		unstruct, err := declarative.GetObjectFromCluster(object, k.reconciler)
		if err != nil {
			log.WithValues("object", object.Kind+"/"+object.Name).Error(err, "Unable to get status of object")
			return err
		}

		res, err := status.Compute(unstruct)
		if err != nil {
			log.WithValues("kind", object.Kind).WithValues("name", object.Name).WithValues("status", res.Status).WithValues(
				"message", res.Message).Info("Got status of resource:")
			statusMap[status.NotFoundStatus] = true
		}
		if res != nil {
			log.WithValues("kind", object.Kind).WithValues("name", object.Name).WithValues("status", res.Status).WithValues("message", res.Message).Info("Got status of resource:")
			statusMap[res.Status] = true
		}
	}

	aggregatedPhase := string(aggregateStatus(statusMap))

	currentStatus, err := utils.GetCommonStatus(src)
	if err != nil {
		log.Error(err, "error retrieving status")
		return err
	}
	if currentStatus.Phase != aggregatedPhase {
		currentStatus.Phase = aggregatedPhase
		err := utils.SetCommonStatus(src, currentStatus)
		if err != nil {
			return err
		}
		log.WithValues("name", src.GetName()).WithValues("phase", aggregatedPhase).Info("updating status")
		err = k.client.Status().Update(ctx, src)
		if err != nil {
			log.Error(err, "error updating status")
			return fmt.Errorf("error error status: %v", err)
		}
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
