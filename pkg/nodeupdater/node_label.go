/**
 * Copyright 2020 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//Package nodeupdater ...
package nodeupdater

import (
	"context"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// VpcNodeLabelUpdater ...
type VpcNodeLabelUpdater struct {
	Node                *v1.Node
	K8sClient           *kubernetes.Clientset
	Logger              *zap.Logger
	StorageSecretConfig *StorageSecretConfig
}

// UpdateNodeLabel gets the details of the newly added node from riaas and updates the labels.
// Returns false and err as nil if labels not updated. else returns true
func (c *VpcNodeLabelUpdater) UpdateNodeLabel(ctx context.Context, workerNodeName string) (done bool, err error) {
	nodeinfo, err := c.GetWorkerDetails(workerNodeName)
	if err != nil {
		return false, err
	}

	// Are adding both worker-id and instance-id label to satisfy all environements.
	// TODO: remove worker-id label after its dependence is removed.
	c.Node.ObjectMeta.Labels[workerIDLabelKey] = nodeinfo.InstanceID
	c.Node.ObjectMeta.Labels[instanceIDLabelKey] = nodeinfo.InstanceID
	c.Node.ObjectMeta.Labels[failureRegionLabelKey] = nodeinfo.Region
	c.Node.ObjectMeta.Labels[failureZoneLabelKey] = nodeinfo.Zone
	c.Node.ObjectMeta.Labels[topologyRegionLabelKey] = nodeinfo.Region
	c.Node.ObjectMeta.Labels[topologyZoneLabelKey] = nodeinfo.Zone
	c.Node.ObjectMeta.Labels[vpcBlockLabelKey] = "true"

	_, err = c.K8sClient.CoreV1().Nodes().Update(ctx, c.Node, metav1.UpdateOptions{})
	if err == nil && !errors.IsConflict(err) {
		c.Logger.Info("Added required labels for the node, ", zap.Reflect("workerNodeName", workerNodeName))
		return true, nil
	}

	return false, err
}
