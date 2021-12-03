#!/bin/bash

set -e
set +x
#git config --global url."https://$GHE_TOKEN@github.ibm.com/".insteadOf "https://github.ibm.com/"
set -x
cd /go/src/github.com/IBM/vpc-node-label-updater
CGO_ENABLED=0 go build -a -ldflags '-X main.vendorVersion='"vpcNodeLabelUpdater-${TAG}"' -extldflags "-static"' -o /go/bin/vpc-node-label-updater ./cmd/
