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
	"encoding/base64"
	"encoding/json"
	errors "errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/IBM/ibmcloud-volume-interface/config"
	"github.com/IBM/ibmcloud-volume-interface/provider/iam"
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

// ReadStorageSecretConfiguration ...
func ReadStorageSecretConfiguration(ctxLogger *zap.Logger) (*StorageSecretConfig, error) {
	ctxLogger.Info("Fetching secret configuration.")
	configPath := filepath.Join(config.GetConfPathDir(), configFileName)
	conf, err := readConfig(configPath, ctxLogger)
	if err != nil {
		ctxLogger.Info("Error loading secret configuration")
		return nil, err
	}

	// Decode g2 API Key if it is a satellite cluster.(unmanaged cluster)
	if os.Getenv(strings.ToUpper("IKS_ENABLED")) != "True" && os.Getenv(strings.ToUpper("IS_SATELLITE")) == "True" {
		ctxLogger.Info("Decoding apiKey since its a satellite cluster")
		apiKey, err := base64.StdEncoding.DecodeString(conf.VPC.G2APIKey)
		if err != nil {
			return nil, err
		}
		conf.VPC.G2APIKey = string(apiKey)
	}

	// Correct if the G2EndpointURL is of the form "http://".
	conf.VPC.G2EndpointURL = getEndpointURL(conf.VPC.G2EndpointURL, ctxLogger)

	// Correct if the G2TokenExchangeURL is of the form "http://"
	conf.VPC.G2TokenExchangeURL = getEndpointURL(conf.VPC.G2TokenExchangeURL, ctxLogger)

	riaasInstanceURL, err := url.Parse(fmt.Sprintf("%s/v1/instances?generation=%s&version=%s", conf.VPC.G2EndpointURL, vpcGeneration, vpcRiaasVersion))
	if err != nil {
		ctxLogger.Error("Failed to parse riassInstanceURL", zap.Error(err))
		return nil, err
	}

	storageSecretConfig := &StorageSecretConfig{
		APIKey:              conf.VPC.G2APIKey,
		IamTokenExchangeURL: fmt.Sprintf("%s/oidc/token", conf.VPC.G2TokenExchangeURL),
		RiaasEndpointURL:    riaasInstanceURL,
		BasicAuthString:     fmt.Sprintf("%s:%s", conf.VPC.IamClientID, conf.VPC.IamClientSecret),
	}

	accessToken, err := storageSecretConfig.GetAccessToken(ctxLogger)
	if err != nil {
		ctxLogger.Error("Failed to Get IAM access token", zap.Error(err))
		return nil, err
	}
	storageSecretConfig.IAMAccessToken = accessToken
	return storageSecretConfig, nil
}

// GetAccessToken ...
func (secretConfig *StorageSecretConfig) GetAccessToken(ctxLogger *zap.Logger) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	form.Set("apikey", secretConfig.APIKey)

	client := &http.Client{}
	req, err := http.NewRequest("POST", secretConfig.IamTokenExchangeURL, strings.NewReader(form.Encode())) // URL-encoded payload
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(secretConfig.BasicAuthString))))
	req.Header.Add("Accept", "application/json")

	var res *http.Response
	err = ErrorRetry(ctxLogger, func() (error, bool) {
		res, err = client.Do(req)               //nolint
		return err, !iam.IsConnectionError(err) // Skip retry if its not connection error
	})
	if err != nil {
		return "", err
	}

	if res == nil || res.StatusCode != 200 {
		ctxLogger.Error("IAM token exchange request failed")
		return "", fmt.Errorf("status Code: %v, check API key providied", res.StatusCode)
	}

	// read response body
	accessTokenRes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		ctxLogger.Error("failed to read response body for getting access token in exchange of apikey", zap.Error(err))
		return "", err
	}
	defer res.Body.Close()
	var accessToken AccessTokenResponse
	err = json.Unmarshal(accessTokenRes, &accessToken)
	if err != nil {
		return "", errors.New("failed to unmarshal json response for access token")
	}
	ctxLogger.Info("Successfully got access token in exchange of apikey")
	return accessToken.AccessToken, nil
}

func readConfig(confPath string, logger *zap.Logger) (*config.Config, error) {
	// load the default config, if confPath not provided
	if confPath == "" {
		confPath = config.GetDefaultConfPath()
	}

	// Parse config file
	conf := config.Config{
		IKS: &config.IKSConfig{}, // IKS block may not be populated in secret toml. Make sure its not nil
	}
	logger.Info("parsing conf file", zap.String("confpath", confPath))
	err := parseConfig(confPath, &conf, logger)
	return &conf, err
}

func parseConfig(filePath string, conf interface{}, logger *zap.Logger) error {
	_, err := toml.DecodeFile(filePath, conf)
	if err != nil {
		logger.Error("Failed to parse config file", zap.Error(err))
	}
	return err
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
