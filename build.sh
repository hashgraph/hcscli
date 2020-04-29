#!/bin/bash

VERSION=$(git describe --dirty=* 2>/dev/null || git describe --always --abbrev=8 --dirty=* || echo "deveopment")
go build -ldflags "-X main.version=$VERSION"
