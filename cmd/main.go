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

//Package main ...
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	nodeupdater "github.com/IBM/vpc-node-label-updater/pkg/nodeupdater"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeu "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	logger *zap.Logger
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	logger = setUpLogger()
	defer logger.Sync()
}

func setUpLogger() *zap.Logger {
	// Prepare a new logger
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller()).With(zap.String("watcher-name", "vpc-node-label-updater"))

	atom.SetLevel(zap.InfoLevel)
	return logger
}

// GetClientConfig first tries to get a config object which uses the service account kubernetes gives to pods,
// if it is called from a process running in a kubernetes environment.
// Otherwise, it tries to build config from a default kubeconfig filepath if it fails, it fallback to the default config.
// Once it get the config, it returns the same.
func GetClientConfig(ctxLogger *zap.Logger) (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		ctxLogger.Error("Failed to create config. Error", zap.Error(err))
		err1 := err
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			err = fmt.Errorf("inClusterConfig as well as BuildConfigFromFlags Failed. Error in InClusterConfig: %+v\nError in BuildConfigFromFlags: %+v", err1, err)
			return nil, err
		}
	}

	return config, nil
}

// GetClientset first tries to get a config object which uses the service account kubernetes gives to pods,
// if it is called from a process running in a kubernetes environment.
// Otherwise, it tries to build config from a default kubeconfig filepath if it fails, it fallback to the default config.
// Once it get the config, it creates a new Clientset for the given config and returns the clientset.
func GetClientset(ctxLogger *zap.Logger) (*kubernetes.Clientset, error) {
	config, err := GetClientConfig(ctxLogger)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		err = fmt.Errorf("failed creating kubernetes clientset. Error: %+v", err)
		return nil, err
	}

	return clientset, nil
}

func main() {
	logger.Info("Starting controller for adding node labels")
	k8sClientset, err := GetClientset(logger)
	if err != nil {
		logger.Fatal("Failed to kubernetes create client set", zap.Error(err))
	}
	nodeName := os.Getenv("NODE_NAME")

	// Do multiple retries to get node details.
	logger.Info("Getting node details")
	var node *v1.Node
	errRetry := nodeupdater.ErrorRetry(logger, func() (error, bool) {
		node, err = k8sClientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			runtimeu.HandleError(fmt.Errorf("node '%s' no longer exist in the cluster", nodeName))
			return err, true // Skip retry if node doesnot exist.
		}
		if err != nil {
			return err, false // Continue retry if error is there.
		}
		return nil, true
	})
	if errRetry != nil || node == nil {
		logger.Fatal("Failed to get node details. Error :", zap.Error(errRetry))
	}

	if nodeupdater.CheckIfRequiredLabelsPresent(node.ObjectMeta.Labels) {
		logger.Info("Required labels already present on the worker node")
		return
	}

	var secretConfig *nodeupdater.StorageSecretConfig
	if secretConfig, err = nodeupdater.ReadStorageSecretConfiguration(logger); err != nil {
		logger.Fatal("Failed to read secret configuration from storage secret present in the cluster ", zap.Error(err))
	}
	c := &nodeupdater.VpcNodeLabelUpdater{
		Node:                node,
		K8sClient:           k8sClientset,
		Logger:              logger,
		StorageSecretConfig: secretConfig,
	}
	if _, err := c.UpdateNodeLabel(context.TODO(), nodeName); err != nil {
		logger.Fatal("error in updating labels for node", zap.Reflect("workerNodeName", nodeName), zap.Error(err))
	}
}
