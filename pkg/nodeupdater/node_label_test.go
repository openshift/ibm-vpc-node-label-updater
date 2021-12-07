/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2021 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

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
