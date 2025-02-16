// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"strings"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

type flowMutator struct {
	logger logr.Logger
	client client.Client
}

// NewFlowMutator returns a new Infrastructure flowMutator that uses mutateFunc to perform the mutation.
func newFlowMutator(mgr manager.Manager, logger logr.Logger) extensionswebhook.Mutator {
	return &flowMutator{
		client: mgr.GetClient(),
		logger: logger,
	}
}

// Mutate mutates the given object on creation and adds the annotation `azure.provider.extensions.gardener.cloud/use-flow=true`
// if the seed has the label `azure.provider.extensions.gardener.cloud/use-flow` == `new`.
func (m *flowMutator) Mutate(ctx context.Context, new, old client.Object) error {
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	newInfra, ok := new.(*extensionsv1alpha1.Infrastructure)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type Infrastructure")
	}

	if m.isInMigrationOrRestorePhase(newInfra) {
		return nil
	}

	gctx := extensionscontextwebhook.NewGardenContext(m.client, new)
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	// skip if shoot is being deleted
	if cluster.Shoot.DeletionTimestamp != nil {
		return nil
	}

	if newInfra.Annotations == nil {
		newInfra.Annotations = map[string]string{}
	}
	if cluster.Seed.Annotations == nil {
		cluster.Seed.Annotations = map[string]string{}
	}
	if cluster.Shoot.Annotations == nil {
		cluster.Shoot.Annotations = map[string]string{}
	}

	mutated := false
	if v, ok := cluster.Shoot.Annotations[azure.GlobalAnnotationKeyUseFlow]; ok {
		newInfra.Annotations[azure.AnnotationKeyUseFlow] = v
		mutated = true
	} else if v, ok := cluster.Shoot.Annotations[azure.AnnotationKeyUseFlow]; ok {
		newInfra.Annotations[azure.AnnotationKeyUseFlow] = v
		mutated = true
	} else if old == nil && cluster.Seed.Annotations[azure.SeedAnnotationKeyUseFlow] == azure.SeedAnnotationUseFlowValueNew {
		newInfra.Annotations[azure.AnnotationKeyUseFlow] = "true"
		mutated = true
	} else if v := cluster.Seed.Annotations[azure.SeedAnnotationKeyUseFlow]; strings.EqualFold(v, "true") {
		newInfra.Annotations[azure.AnnotationKeyUseFlow] = "true"
		mutated = true
	}

	if mutated {
		extensionswebhook.LogMutation(logger, newInfra.Kind, newInfra.Namespace, newInfra.Name)
	}

	return nil
}

func (m *flowMutator) isInMigrationOrRestorePhase(infra *extensionsv1alpha1.Infrastructure) bool {
	operationType := helper.ComputeOperationType(infra.ObjectMeta, infra.Status.LastOperation)

	return operationType == v1beta1.LastOperationTypeMigrate || operationType == v1beta1.LastOperationTypeRestore
}
