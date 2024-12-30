#!/bin/bash

echo "Run deploy script"
echo "nameserver 8.8.8.8" | sudo tee /etc/resolv.conf
echo "nameserver 1.1.1.1" | sudo tee -a /etc/resolv.conf
docker pull ghcr.io/filinvadim/warpnet:latest
docker run -d --name warpnet -p 4001:4001 -p 4002:4002 ghcr.io/filinvadim/warpnet:latest
