#!/bin/bash
go build -ldflags "-X main.Version=$(cat VERSION) -X main.BuildTime=$(date -u +%Y%m%d)" -v 
sudo docker build -t "dunze/ygobuster:$(cat VERSION)" -f Dockerfile .

