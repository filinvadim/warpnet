#!/bin/bash

# Убедитесь, что обновления установлены
sudo apt update && sudo apt upgrade -y

# Установите Docker (если не установлен)
if ! [ -x "$(command -v docker)" ]; then
  curl -fsSL https://get.docker.com -o get-docker.sh
  sh get-docker.sh
  sudo usermod -aG docker $USER
fi

# Скачайте образ приложения из GitHub Container Registry
docker pull ghcr.io/filinvadim/warpnet:latest

docker run -d --name warpnet -p 4001:4001 -p 4002:4002 ghcr.io/filinvadim/warpnet:latest
