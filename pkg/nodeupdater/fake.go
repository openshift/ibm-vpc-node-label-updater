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
	"bytes"
	"context"
	errors "errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MockVPCNodeLabelUpdater ...
type MockVPCNodeLabelUpdater struct {
	Node                *v1.Node
	K8sClient           *kubernetes.Clientset
	Logger              *zap.Logger
	StorageSecretConfig *StorageSecretConfig
}

// GetTestLogger ...
func GetTestLogger(t *testing.T) (logger *zap.Logger, teardown func()) {
	atom := zap.NewAtomicLevel()
	atom.SetLevel(zap.DebugLevel)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	buf := &bytes.Buffer{}

	logger = zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(buf),
			atom,
		),
		zap.AddCaller(),
	)

	teardown = func() {
		_ = logger.Sync() // #nosec G104: flush any buffered log entries only on best-effort basis.Error cannot be usefully handled.
		if t.Failed() {
			t.Log(buf)
		}
	}
	return
}

func initMockNodeLabelUpdater(t *testing.T) *MockVPCNodeLabelUpdater {
	logger, teardown := GetTestLogger(t)
	defer teardown()
	mockVPCNodeLabelUpdater := &MockVPCNodeLabelUpdater{
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

// GetWorkerDetails ...
func (m *MockVPCNodeLabelUpdater) GetWorkerDetails(workerNodeName string) (*NodeInfo, error) {
	if strings.Contains(workerNodeName, "ip") {
		return m.GetInstanceByIP(workerNodeName)
	}
	return m.GetInstanceByName(workerNodeName)
}

// GetInstancesFromVPC ...
func (m *MockVPCNodeLabelUpdater) GetInstancesFromVPC(riaasInstanceURL *url.URL) ([]*Instance, error) {
	fmt.Println(m.StorageSecretConfig.IAMAccessToken)
	if m.StorageSecretConfig.IAMAccessToken == "" {
		return nil, errors.New("failed to get worker details as instance list is empty")
	}
	if riaasInstanceURL.Scheme == "" {
		return nil, errors.New("get \"\": unsupported protocol scheme \"\"")
	}
	if strings.Contains(riaasInstanceURL.Host, "invalid") {
		return nil, errors.New("get \"https://invalid\": dial tcp: lookup invalid: Temporary failure in name resolution")
	}
	ins := &Instance{
		Name: "valid-worker",
		ID:   "valid-instance-id",
		PrimaryNetworkInterface: &NetworkInterface{
			PrimaryIpv4Address: "valid-worker-ip",
		},
	}
	insL := []*Instance{ins}
	return insL, nil
}

// GetInstanceByName ...
func (m *MockVPCNodeLabelUpdater) GetInstanceByName(workerNodeName string) (*NodeInfo, error) {
	if strings.Contains(workerNodeName, "invalid") {
		return nil, errors.New("failed to get worker details as instance list is empty")
	}
	insL, err := m.GetInstancesFromVPC(m.StorageSecretConfig.RiaasEndpointURL)
	if err != nil {
		return nil, err
	}
	return m.getNodeInfo(insL[0])
}

// GetInstanceByIP ...
func (m *MockVPCNodeLabelUpdater) GetInstanceByIP(workerNodeName string) (*NodeInfo, error) {
	if strings.Contains(workerNodeName, "invalid-ip") {
		return nil, errors.New("failed to get worker details as instance list is empty")
	}
	if m.StorageSecretConfig.IAMAccessToken == "" {
		return nil, errors.New("failed to get worker details as instance list is empty")
	}
	if m.StorageSecretConfig.RiaasEndpointURL.Scheme == "" {
		return nil, errors.New("get \"\": unsupported protocol scheme \"\"")
	}
	if strings.Contains(m.StorageSecretConfig.RiaasEndpointURL.Host, "invalid") {
		return nil, errors.New("get \"https://invalid\": dial tcp: lookup invalid: Temporary failure in name resolution")
	}
	if strings.Contains(workerNodeName, "valid-worker-ip") {
		return &NodeInfo{}, nil
	}
	return nil, errors.New("")
}

func (m *MockVPCNodeLabelUpdater) getNodeInfo(instance *Instance) (*NodeInfo, error) {
	if instance == nil {
		return nil, errors.New("instance is nil")
	}
	return &NodeInfo{
		InstanceID: instance.ID,
		Zone:       "valid-zone",
	}, nil
}

// UpdateNodeLabel ...
func (m *MockVPCNodeLabelUpdater) UpdateNodeLabel(ctx context.Context, workerNodeName string) (done bool, err error) {
	if strings.Contains(workerNodeName, "valid") {
		return true, nil
	}
	return false, errors.New("")
}
