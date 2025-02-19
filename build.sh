#!/bin/bash

PLATFORMS="linux,amd64 linux,arm64 darwin,amd64 darwin,arm64"


for platform in $PLATFORMS
do
	export GOOS=$(echo $platform | cut -d',' -f1)
	export GOARCH=$(echo $platform | cut -d',' -f2)

	go build -o config-watcher-${GOOS}-${GOARCH}
done
