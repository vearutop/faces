#!/bin/bash

set -e

docker build -f docker/Dockerfile.ubuntu20.04 -t dlibubuntu .
docker build -f docker/Dockerfile.go-face -t goface .
docker build -f docker/Dockerfile.app -t faces .