package status

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

type kstatusAggregator struct {
	client client.Client
	reconciler *declarative.Reconciler
}

func NewKstatusAgregator(c client.Client, reconciler *declarative.Reconciler) *kstatusAggregator{
	return &kstatusAggregator{client: c, reconciler: reconciler}
}

func(k *kstatusAggregator) Reconciled(ctx context.Context, src declarative.DeclarativeObject,
	objs *manifest.Objects) error{
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

	addonObject, ok := src.(addonsv1alpha1.CommonObject)
	unstruct, unstructOk := src.(*unstructured.Unstructured)
	aggregatedPhase := string(aggregateStatus(statusMap))
	changed := false
	if ok {
		addonStatus := addonObject.GetCommonStatus()
		if addonStatus.Phase != aggregatedPhase {
			addonStatus.Phase = aggregatedPhase
			addonObject.SetCommonStatus(addonStatus)
			changed = true
		}
	} else if unstructOk {
		statusPhase, _, err := unstructured.NestedString(unstruct.Object, "status", "phase")
		if err != nil {
			log.Error(err, "error retrieving status")
			return err
		}

		if statusPhase != aggregatedPhase {
			err := unstructured.SetNestedField(unstruct.Object, aggregatedPhase, "status", "phase")
			if err != nil {
				log.Error(err, "error retrieving status")
				return err
			}
			changed = true
		}
	} else {
		return fmt.Errorf("instance %T was not an addonsv1alpha1.CommonObject or unstructured." +
			"Unstructured",
			src)
	}

	if changed == true {
		log.WithValues("name", src.GetName()).WithValues("phase", aggregatedPhase).Info("updating status")
		err := k.client.Status().Update(ctx, src)
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
