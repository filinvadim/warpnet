#!/bin/bash

echo "Run deploy script"
echo "nameserver 8.8.8.8" > /etc/resolv.conf
echo "nameserver 1.1.1.1" >> /etc/resolv.conf
echo "GITHUB_TOKEN: ${GITHUB_TOKEN:0:4}... (truncated for security)"
if [ -z "$GITHUB_TOKEN" ]; then
  echo "Error: GITHUB_TOKEN is not set"
  exit 1
fi
docker stop $(docker ps -aq) || true
docker container prune -f
echo $GITHUB_TOKEN | docker login ghcr.io -u filinvadim --password-stdin
docker pull ghcr.io/filinvadim/warpnet:latest
docker run --restart=always -d --network=host -h "$HOST" -e PUBLIC_IP="$HOST" --name warpnet --privileged ghcr.io/filinvadim/warpnet:latest
