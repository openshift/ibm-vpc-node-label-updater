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
	"encoding/json"
	errors "errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/IBM/ibmcloud-volume-interface/provider/iam"
	sp "github.com/IBM/secret-common-lib/pkg/secret_provider"
	"go.uber.org/zap"
)

const (
	workerIDLabelKey       = "ibm-cloud.kubernetes.io/worker-id"
	instanceIDLabelKey     = "ibm-cloud.kubernetes.io/vpc-instance-id"
	failureRegionLabelKey  = "failure-domain.beta.kubernetes.io/region"
	failureZoneLabelKey    = "failure-domain.beta.kubernetes.io/zone"
	topologyRegionLabelKey = "topology.kubernetes.io/region"
	topologyZoneLabelKey   = "topology.kubernetes.io/zone"
	configFileName         = "slclient.toml"
	vpcGeneration          = "2"
	vpcRiaasVersion        = "2020-01-01"
	maxAttempts            = 30
	retryInterval          = "10s"
	vpcBlockLabelKey       = "vpc-block-csi-driver-labels"
)

// ReadSecretConfiguration ...
func ReadSecretConfiguration(ctxLogger *zap.Logger) (*StorageSecretConfig, error) {
	ctxLogger.Info("Fetching secret configuration.")
	providerType := map[string]string{
		sp.ProviderType: sp.VPC,
	}
	spObject, err := sp.NewSecretProvider(providerType)
	if err != nil {
		ctxLogger.Error("Error initializing secret provider", zap.Error(err))
		return nil, err
	}

	riaasURL, err := spObject.GetRIAASEndpoint(false)
	if err != nil {
		ctxLogger.Error("Error fetching RIAAS endpoint", zap.Error(err))
		return nil, err
	}

	// Correct if the G2EndpointURL is of the form "http://".
	riaasURL = getEndpointURL(riaasURL, ctxLogger)
	riaasInstanceURL, err := url.Parse(fmt.Sprintf("%s/v1/instances?generation=%s&version=%s", riaasURL, vpcGeneration, vpcRiaasVersion))
	if err != nil {
		ctxLogger.Error("Failed to parse riassInstanceURL", zap.Error(err))
		return nil, err
	}
	storageSecretConfig := &StorageSecretConfig{
		RiaasEndpointURL: riaasInstanceURL,
	}

	accessToken, _, err := spObject.GetDefaultIAMToken(false, "vpc-node-label-updater")
	if err != nil {
		ctxLogger.Error("Failed to Get IAM access token", zap.Error(err))
		return nil, err
	}
	storageSecretConfig.IAMAccessToken = accessToken
	return storageSecretConfig, nil
}

// ErrorRetry ...
func ErrorRetry(logger *zap.Logger, funcToRetry func() (error, bool)) error {
	var err error
	var shouldStop bool
	retryIntervaltime, err := time.ParseDuration(retryInterval)
	if err != nil {
		logger.Warn("time.ParseDuration failed", zap.Error(err))
	}
	for i := 0; ; i++ {
		err, shouldStop = funcToRetry()
		logger.Debug("Retry Function Result", zap.Error(err), zap.Bool("shouldStop", shouldStop))
		if shouldStop {
			break
		}
		if err == nil {
			return err
		}
		//Stop if out of retries
		if i >= (maxAttempts - 1) {
			break
		}
		time.Sleep(retryIntervaltime)
		logger.Warn("retrying after Error:", zap.Error(err))
	}
	//error set by name above so no need to explicitly return it
	return err
}

// CheckIfRequiredLabelsPresent checks if nodes are already labeled with the required labels
func CheckIfRequiredLabelsPresent(labelMap map[string]string) bool {
	_, okvpcBlockLabelKey := labelMap[vpcBlockLabelKey]
	_, okvpcInstanceID := labelMap[instanceIDLabelKey]
	/* For users using version <=4.2.2, need to check for both label vpcBlockLabelKey and instanceIDLabelKey
	TODO: Keep only check for vpcBlockLabelKey when version 4.2.2 is removed
	*/
	if okvpcBlockLabelKey && okvpcInstanceID {
		return true
	}
	return false
}

// getEndpointURL corrects endpoint url if it is of form "http://"
func getEndpointURL(url string, logger *zap.Logger) string {
	if strings.Contains(url, "http://") {
		logger.Warn("Token exchange endpoint URL is of the form 'http' instead 'https'. Correcting it for valid request.", zap.Reflect("Endpoint URL: ", url))
		return strings.Replace(url, "http", "https", 1)
	}
	return url
}

// GetWorkerDetails ...
func (c *VpcNodeLabelUpdater) GetWorkerDetails(workerNodeName string) (*NodeInfo, error) {
	if net.ParseIP(workerNodeName) == nil {
		c.Logger.Info("Worker Node Name is not in ip format. Getting instance detail by name from vpc provider")
		return c.GetInstanceByName(workerNodeName)
	}
	c.Logger.Info("Worker Node Name is in ip format. Getting instance detail by ipv4 from vpc provider")
	return c.GetInstanceByIP(workerNodeName)
}

// GetInstancesFromVPC ...
func (c *VpcNodeLabelUpdater) GetInstancesFromVPC(riaasInstanceURL *url.URL) ([]*Instance, error) {
	c.Logger.Info("Getting instance List from VPC provider")

	instanceReq := &http.Request{
		Method: "GET",
		URL:    riaasInstanceURL,
		Header: map[string][]string{
			"Content-Type":  {"application/json"},
			"Accept":        {"application/json"},
			"Authorization": {c.StorageSecretConfig.IAMAccessToken},
		},
	}
	var instanceResponse *http.Response
	var err error

	err = ErrorRetry(c.Logger, func() (error, bool) {
		instanceResponse, err = http.DefaultClient.Do(instanceReq) //nolint
		return err, !iam.IsConnectionError(err)                    // Skip retry if its not connection error
	})

	if err != nil {
		return nil, err
	}
	defer instanceResponse.Body.Close()
	// read response body
	instance, err := ioutil.ReadAll(instanceResponse.Body)
	if err != nil {
		c.Logger.Error("Failed to read response body of instance details from riaas provider", zap.Error(err))
		return nil, err
	}
	var instanceList InstanceList
	err = json.Unmarshal(instance, &instanceList)
	if err != nil {
		return nil, errors.New("failed to unmarshal json response of instances")
	}
	if len(instanceList.Instances) == 0 {
		return nil, errors.New("failed to get worker details as instance list is empty")
	}
	return instanceList.Instances, nil
}

// GetInstanceByIP ...
func (c *VpcNodeLabelUpdater) GetInstanceByIP(workerNodeName string) (*NodeInfo, error) {
	c.Logger.Info("Getting InstanceList from VPC provider...")

	instanceList, err := c.GetInstancesFromVPC(c.StorageSecretConfig.RiaasEndpointURL)
	if err != nil {
		return nil, err
	}

	for _, instanceItem := range instanceList {
		// Check if worker IP is matching with requested worker node name
		if instanceItem.PrimaryNetworkInterface.PrimaryIpv4Address == workerNodeName {
			c.Logger.Info("Successfully found instance", zap.Reflect("instanceDetail", instanceItem))
			return c.getNodeInfo(instanceItem), nil
		}
	}
	err = fmt.Errorf("failed to get worker details, worker with name %s was not found in the instanceList fetched from vpc provider", workerNodeName)
	return nil, err
}

// GetInstanceByName ...
func (c *VpcNodeLabelUpdater) GetInstanceByName(workerNodeName string) (*NodeInfo, error) {
	c.Logger.Info("Getting InstanceList from VPC provider...")

	riaasInstanceURL := c.StorageSecretConfig.RiaasEndpointURL
	q := riaasInstanceURL.Query()
	q.Add("name", workerNodeName)
	c.StorageSecretConfig.RiaasEndpointURL.RawQuery = q.Encode()

	instanceList, err := c.GetInstancesFromVPC(riaasInstanceURL)
	if err != nil {
		return nil, err
	}

	return c.getNodeInfo(instanceList[0]), nil
}

func (c *VpcNodeLabelUpdater) getNodeInfo(instance *Instance) *NodeInfo {
	insID := instance.ID
	zone := instance.Zone.Name
	lastInd := strings.LastIndex(zone, "-")
	region := zone[:lastInd]

	nodeDetails := &NodeInfo{
		InstanceID: insID,
		Zone:       zone,
		Region:     region,
	}
	c.Logger.Info("Successfully fetched node detail from VPC provider", zap.Reflect("nodeDetails", nodeDetails))
	return nodeDetails
}
