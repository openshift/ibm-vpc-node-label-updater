FROM golang:1.19.10

WORKDIR /go/src/github.com/IBM/vpc-node-label-updater
ADD . /go/src/github.com/IBM/vpc-node-label-updater

ARG TAG
ARG OS
ARG ARCH

CMD ["./scripts/build-bin.sh"]
