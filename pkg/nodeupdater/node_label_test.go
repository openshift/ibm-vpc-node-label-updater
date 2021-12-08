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
	errors "errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateNodeLabel(t *testing.T) {
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
			accessToken:      "",
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
			workerNodeName:   "",
			accessToken:      "valid-token",
			expErr:           errors.New("Get \"?name=\": unsupported protocol scheme \"\""), //nolint
		},
		{
			name:             "invalid riaasInstanceURL",
			workerNodeName:   "",
			riaasInstanceURL: "https://invalid",
			accessToken:      "valid-token",
			expErr:           errors.New("Get \"https://invalid?name=\": dial tcp: lookup invalid: Temporary failure in name resolution"), //nolint
		},
	}
	mockupdater := initNodeLabelUpdater(t)

	for _, tc := range testCases {
		if tc.name == "valid Request" {
			mockupdater := initMockNodeLabelUpdater(t)
			_, err := mockupdater.UpdateNodeLabel(context.TODO(), tc.workerNodeName)
			assert.Nil(t, err)
		} else {
			t.Logf("Test case: %s", tc.name)
			riaasInsURL, _ := url.Parse(tc.riaasInstanceURL)
			mockupdater.StorageSecretConfig.IAMAccessToken = tc.accessToken
			mockupdater.StorageSecretConfig.RiaasEndpointURL = riaasInsURL
			_, err := mockupdater.UpdateNodeLabel(context.TODO(), tc.workerNodeName)
			if err != nil {
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("Expected error : %v, got: %v. err : %v", tc.expErr, err, err)
				}
			}
			continue
		}
	}
}
