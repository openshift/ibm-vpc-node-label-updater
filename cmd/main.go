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

	"github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	nodeupdater "github.com/IBM/vpc-node-label-updater/pkg/nodeupdater"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeu "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	logger *zap.Logger
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	logger = setUpLogger()
	defer func() {
		_ = logger.Sync() // #nosec G104: Attempt to logg sync only on best-effort basis.Error cannot be usefully handled.
	}()
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

func main() {
	logger.Info("Starting controller for adding node labels")
	k8sClient, err := k8s_utils.Getk8sClientSet()
	if err != nil {
		logger.Fatal("Failed to kubernetes create client set", zap.Error(err))
	}
	nodeName := os.Getenv("NODE_NAME")

	// Do multiple retries to get node details.
	logger.Info("Getting node details")
	var node *v1.Node
	errRetry := nodeupdater.ErrorRetry(logger, func() (error, bool) {
		node, err = k8sClient.Clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
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
	if secretConfig, err = nodeupdater.ReadSecretConfiguration(&k8sClient, logger); err != nil {
		logger.Fatal("Failed to read secret configuration", zap.Error(err))
	}
	c := &nodeupdater.VpcNodeLabelUpdater{
		Node:                node,
		K8sClient:           k8sClient.Clientset,
		Logger:              logger,
		StorageSecretConfig: secretConfig,
	}
	if _, err := c.UpdateNodeLabel(context.TODO(), nodeName); err != nil {
		logger.Fatal("error in updating labels for node", zap.Reflect("workerNodeName", nodeName), zap.Error(err))
	}
}
