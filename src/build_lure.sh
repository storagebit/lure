#!/bin/bash
#MIT License
#
#Copyright (c) 2020 storagebit.ch
#
#Permission is hereby granted, free of charge, to any person obtaining a copy
#of this software and associated documentation files (the "Software"), to deal
#in the Software without restriction, including without limitation the rights
#to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
#copies of the Software, and to permit persons to whom the Software is
#furnished to do so, subject to the following conditions:
#
#The above copyright notice and this permission notice shall be included in all
#copies or substantial portions of the Software.
#
#THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
#IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
#FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
#AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
#LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
#OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
#SOFTWARE.

# Build script for lure
# Did you install the golang dev environment?
# Easy test is running 'go version' - if that works you should be good.

# First, lets get the build dependencies installed
echo -e "Checking if github.com/buger/goterm package is available and installing it if necessary."
go get -v github.com/buger/goterm
echo -e "Checking if the github.com/dustin/go-humanize package is available and installing it if necessary."
go get -v github.com/dustin/go-humanize

# Build the executable
echo -e "Compiling the binary/executable"
go build -x -o ../bin/lure -ldflags "-X main.buildSha1=$(git rev-parse HEAD)  \
                                  -X main.buildBranch=$(git rev-parse --abbrev-ref HEAD) \
                                  -X main.buildTime=$(date +'%Y-%m-%d_%T')"
