#!/bin/bash
go build -o ../bin/lure -ldflags "-X main.buildSha1=`git rev-parse HEAD`  \
                                  -X main.buildBranch=`git rev-parse --abbrev-ref HEAD` \
                                  -X main.buildTime=`date +'%Y-%m-%d_%T'`"
