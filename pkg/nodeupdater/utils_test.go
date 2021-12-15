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
	errors "errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func initNodeLabelUpdater(t *testing.T) *VpcNodeLabelUpdater {
	logger, teardown := GetTestLogger(t)
	defer teardown()
	mockVPCNodeLabelUpdater := &VpcNodeLabelUpdater{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-node",
				Namespace: "fake",
				Labels:    map[string]string{"test": "test"},
			},
		},
		Logger:              logger,
		StorageSecretConfig: &StorageSecretConfig{},
		K8sClient:           nil,
	}

	return mockVPCNodeLabelUpdater
}

func TestReadStorageSecretConfiguration(t *testing.T) {
	// Creating test logger
	logger, teardown := GetTestLogger(t)
	defer teardown()

	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get current working directory, some unit tests will fail")
	}

	// As its required by NewIBMCloudStorageProvider
	secretConfigPath := filepath.Join(pwd, "..", "..", "test-fixtures")
	err = os.Setenv("SECRET_CONFIG_PATH", secretConfigPath)
	defer os.Unsetenv("SECRET_CONFIG_PATH")
	if err != nil {
		t.Errorf("This test will fail because of %v", err)
	}

	_, err = ReadStorageSecretConfiguration(logger)
	assert.NotNil(t, err)
}

func TestGetAccessToken(t *testing.T) {
	// Creating test logger
	logger, teardown := GetTestLogger(t)
	defer teardown()
	testCases := []struct {
		name         string
		secretConfig *StorageSecretConfig
		expErr       error
	}{
		{
			name: "valid Request",

			secretConfig: &StorageSecretConfig{
				IamTokenExchangeURL: "https://iam.bluemix.net/oidc/token",
				APIKey:              "ghytfyhgj",
				BasicAuthString:     fmt.Sprintf("%s:%s", "bx", "bx"),
			},
			expErr: nil,
		},
		{
			name: "Empty IamTokenExchangeURL",
			secretConfig: &StorageSecretConfig{
				IamTokenExchangeURL: "",
			},
			expErr: errors.New("Post \"\": unsupported protocol scheme \"\""), //nolint
		},
		{
			name: "invalid IamTokenExchangeURL",
			secretConfig: &StorageSecretConfig{
				IamTokenExchangeURL: "https://xy",
			},
			expErr: errors.New("Post \"https://xy\": dial tcp: lookup xy"), //nolint
		},
	}
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := tc.secretConfig.GetAccessToken(logger)
		if err != nil {
			if err.Error() != tc.expErr.Error() && !strings.Contains(err.Error(), tc.expErr.Error()) {
				t.Fatalf("Expected error code: %v, got: %v. err : %v", tc.expErr, err, err)
			}
			continue
		}
	}
}

type testConfig struct {
	Header sectionTestConfig
}

type sectionTestConfig struct {
	ID      int
	Name    string
	YesOrNo bool
	Pi      float64
	List    string
}

var testConf = testConfig{
	Header: sectionTestConfig{
		ID:      1,
		Name:    "test",
		YesOrNo: true,
		Pi:      3.14,
		List:    "1, 2",
	},
}

func TestParseConfig(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()
	var testParseConf testConfig

	configPath := "test.toml"
	err := parseConfig(configPath, &testParseConf, logger)
	assert.Nil(t, err)

	expected := testConf
	assert.Exactly(t, expected, testParseConf)
}

func TestParseConfigNoMatch(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()
	var testParseConf testConfig

	configPath := "test.toml"
	err := parseConfig(configPath, &testParseConf, logger)
	assert.Nil(t, err)

	expected := testConfig{
		Header: sectionTestConfig{
			ID:      1,
			Name:    "testnomatch",
			YesOrNo: true,
			Pi:      3.14,
			List:    "1, 2",
		}}

	assert.NotEqual(t, expected, testParseConf)
}

func TestReadConfig(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	configPath := "test.toml"
	expectedConf, _ := readConfig(configPath, logger)

	assert.NotNil(t, expectedConf)
}

func TestReadConfigEmptyPath(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	configPath := ""
	expectedConf, _ := readConfig(configPath, logger)

	assert.NotNil(t, expectedConf)
}

func TestCheckIfRequiredLabelsPresent(t *testing.T) {
	labelMap := make(map[string]string)
	exp := CheckIfRequiredLabelsPresent(labelMap)
	assert.Equal(t, exp, false)
	labelMap[vpcBlockLabelKey] = "true"
	ex := CheckIfRequiredLabelsPresent(labelMap)
	assert.Equal(t, ex, true)
}

func TestGetInstancesFromVPC(t *testing.T) {
	testCases := []struct {
		name             string
		workerNodeName   string
		riaasInstanceURL string
		accessToken      string
		expErr           error
	}{
		{
			name:             "valid Request",
			workerNodeName:   "valid-worker",
			accessToken:      "valid-token",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           nil,
		},
		{
			name:             "empty accessToken",
			workerNodeName:   "valid-worker",
			accessToken:      "",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           errors.New("failed to unmarshal json response of instances"),
		},

		{
			name:             "Empty riaasInstanceURL",
			riaasInstanceURL: "",
			accessToken:      "valid-token",
			expErr:           errors.New("Get \"\": unsupported protocol scheme \"\""), //nolint
		},
		{
			name:             "invalid riaasInstanceURL",
			riaasInstanceURL: "https://invalid",
			accessToken:      "valid-token",
			expErr:           errors.New("Get \"https://invalid\": dial tcp: lookup invalid"), //nolint
		},
	}
	updater := initNodeLabelUpdater(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		riaasInsURL, _ := url.Parse(tc.riaasInstanceURL)
		if tc.name == "valid Request" {
			mockupdater := initMockNodeLabelUpdater(t)
			mockupdater.StorageSecretConfig.IAMAccessToken = tc.accessToken
			_, err := mockupdater.GetInstancesFromVPC(riaasInsURL)
			assert.Nil(t, err)
		} else {
			updater.StorageSecretConfig.IAMAccessToken = tc.accessToken
			_, err := updater.GetInstancesFromVPC(riaasInsURL)
			if err != nil {
				if err.Error() != tc.expErr.Error() && !strings.Contains(err.Error(), tc.expErr.Error()) {
					t.Fatalf("Expected error : %v, got: %v. err : %v", tc.expErr, err, err)
				}
			}
			continue
		}
	}
}

func TestGetInstanceByIP(t *testing.T) {
	testCases := []struct {
		name             string
		workerNodeName   string
		riaasInstanceURL string
		accessToken      string
		expErr           error
	}{
		{
			name:             "valid Request",
			workerNodeName:   "valid-worker-ip",
			accessToken:      "valid-token",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           nil,
		},
		{
			name:             "empty accessToken",
			workerNodeName:   "valid-worker",
			accessToken:      "",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           errors.New("failed to get worker details as instance list is empty"),
		},

		{
			name:             "Empty riaasInstanceURL",
			riaasInstanceURL: "",
			accessToken:      "valid-token",
			expErr:           errors.New("get \"\": unsupported protocol scheme \"\""),
		},
		{
			name:             "invalid riaasInstanceURL",
			riaasInstanceURL: "https://invalid",
			accessToken:      "valid-token",
			expErr:           errors.New("get \"https://invalid\": dial tcp: lookup invalid"),
		},
		{
			name:             "invalid worker-ip",
			workerNodeName:   "invalid-ip",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			accessToken:      "valid-token",
			expErr:           errors.New("failed to get worker details as instance list is empty"),
		},
	}
	mockupdater := initMockNodeLabelUpdater(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		riaasInsURL, _ := url.Parse(tc.riaasInstanceURL)
		mockupdater.StorageSecretConfig.IAMAccessToken = tc.accessToken
		mockupdater.StorageSecretConfig.RiaasEndpointURL = riaasInsURL
		_, err := mockupdater.GetInstanceByIP(tc.workerNodeName)
		if err != nil {
			if err.Error() != tc.expErr.Error() && !strings.Contains(err.Error(), tc.expErr.Error()) {
				t.Fatalf("Expected error : %v, got: %v. err : %v", tc.expErr, err, err)
			}
		}
		continue
	}
}

func TestGetInstanceByName(t *testing.T) {
	testCases := []struct {
		name             string
		workerNodeName   string
		riaasInstanceURL string
		accessToken      string
		expErr           error
	}{
		{
			name:             "valid Request",
			workerNodeName:   "valid-worker",
			accessToken:      "valid-token",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           nil,
		},
		{
			name:             "empty accessToken",
			workerNodeName:   "valid-worker",
			accessToken:      "",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           errors.New("failed to get worker details as instance list is empty"),
		},

		{
			name:             "Empty riaasInstanceURL",
			riaasInstanceURL: "",
			accessToken:      "valid-token",
			expErr:           errors.New("get \"\": unsupported protocol scheme \"\""),
		},
		{
			name:             "invalid riaasInstanceURL",
			riaasInstanceURL: "https://invalid",
			accessToken:      "valid-token",
			expErr:           errors.New("get \"https://invalid\": dial tcp: lookup invalid"),
		},
		{
			name:             "invalid worker",
			workerNodeName:   "invalid-worker",
			riaasInstanceURL: "https://invalid",
			accessToken:      "valid-token",
			expErr:           errors.New("failed to get worker details as instance list is empty"),
		},
	}
	mockupdater := initMockNodeLabelUpdater(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		riaasInsURL, _ := url.Parse(tc.riaasInstanceURL)
		mockupdater.StorageSecretConfig.IAMAccessToken = tc.accessToken
		mockupdater.StorageSecretConfig.RiaasEndpointURL = riaasInsURL
		_, err := mockupdater.GetInstanceByName(tc.workerNodeName)
		if err != nil {
			assert.Equal(t, tc.expErr, err)
		}
		continue
	}
}

func TestGetWorkerDetails(t *testing.T) {
	testCases := []struct {
		name             string
		workerNodeName   string
		riaasInstanceURL string
		accessToken      string
		expErr           error
	}{
		{
			name:             "valid worker name Request",
			workerNodeName:   "valid-worker-name",
			accessToken:      "valid-token",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           nil,
		},
		{
			name:             "valid worker ip Request",
			workerNodeName:   "valid-worker-ip",
			accessToken:      "valid-token",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           nil,
		},
		{
			name:             "empty accessToken",
			workerNodeName:   "valid-worker",
			accessToken:      "",
			riaasInstanceURL: "https://us-south.iaas.cloud.ibm.com",
			expErr:           errors.New("failed to get worker details as instance list is empty"),
		},

		{
			name:             "Empty riaasInstanceURL",
			riaasInstanceURL: "",
			accessToken:      "valid-token",
			expErr:           errors.New("get \"\": unsupported protocol scheme \"\""),
		},
		{
			name:             "invalid riaasInstanceURL",
			riaasInstanceURL: "https://invalid",
			accessToken:      "valid-token",
			expErr:           errors.New("get \"https://invalid\": dial tcp: lookup invalid"),
		},
		{
			name:             "invalid worker",
			workerNodeName:   "invalid-worker",
			riaasInstanceURL: "https://invalid",
			accessToken:      "valid-token",
			expErr:           errors.New("failed to get worker details as instance list is empty"),
		},
	}
	mockupdater := initMockNodeLabelUpdater(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		riaasInsURL, _ := url.Parse(tc.riaasInstanceURL)
		mockupdater.StorageSecretConfig.IAMAccessToken = tc.accessToken
		mockupdater.StorageSecretConfig.RiaasEndpointURL = riaasInsURL
		_, err := mockupdater.GetWorkerDetails(tc.workerNodeName)
		if err != nil {
			assert.Equal(t, tc.expErr, err)
		}
		continue
	}
}

func TestGetNodeInfo(t *testing.T) {
	testCases := []struct {
		name     string
		instance *Instance
		expRes   *NodeInfo
	}{
		{
			name:     "not nil instance",
			instance: &Instance{ID: "instance-id", Zone: &Zone{Name: "xyz-1"}},
			expRes:   &NodeInfo{InstanceID: "instance-id", Region: "xyz", Zone: "xyz-1"},
		},
	}
	mockupdater := initNodeLabelUpdater(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		nodeinfo := mockupdater.getNodeInfo(tc.instance)
		assert.Equal(t, tc.expRes, nodeinfo)
		continue
	}
}
