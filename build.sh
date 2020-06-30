#!/bin/bash

VERSION=$(git describe --dirty=* 2>/dev/null || git describe --always --abbrev=8 --dirty=* || echo "development")
go build -ldflags "-X github.com/hashgraph/hcscli/cmd.Version=$VERSION"
