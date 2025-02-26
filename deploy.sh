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
docker compose down
echo $GITHUB_TOKEN | docker login ghcr.io -u filinvadim --password-stdin
docker pull ghcr.io/filinvadim/warpnet:latest
ufw disable
systemctl restart systemd-networkd
iptables -I DOCKER-USER -j ACCEPT
iptables -A INPUT -p tcp --match multiport --dports 4001:4003 -j ACCEPT
iptables -A FORWARD -i docker0 -o docker0 -j ACCEPT
iptables -A FORWARD -i br-6383b19e4979 -o br-6383b19e4979 -j ACCEPT
iptables -P FORWARD ACCEPT
iptables -A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
mkdir -p ./data || true
touch ./data/snapshot || true
docker compose up -d --build
