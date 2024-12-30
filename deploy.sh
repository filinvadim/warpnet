#!/bin/bash

echo "Run deploy script"
echo "nameserver 8.8.8.8" > /etc/resolv.conf
echo "nameserver 1.1.1.1" >> /etc/resolv.conf
echo "GITHUB_TOKEN: ${GITHUB_TOKEN:0:4}... (truncated for security)"
if [ -z "$GITHUB_TOKEN" ]; then
  echo "Error: GITHUB_TOKEN is not set"
  exit 1
fi
echo $GITHUB_TOKEN | docker login ghcr.io -u filinvadim --password-stdin
docker pull ghcr.io/filinvadim/warpnet:latest
docker run -d --name warpnet -p 4001:4001 -p 4002:4002 ghcr.io/filinvadim/warpnet:latest
