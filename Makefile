oapi-codegen-install:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

gen:
	~/go/bin/oapi-codegen -generate types,spec,skip-prune -package domain ./spec/domain.yml > domain-gen/domain.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/domain-gen -generate server,types,spec,skip-prune -package api ./spec/local-api.yml > server/api-gen/api.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/domain-gen -generate types,spec,skip-prune -package node ./spec/node.yml > core/node-gen/api.gen.go
tests:
	CGO_ENABLED=0 go test -count=1 -short ./...

govendor:
	go mod tidy
	go mod vendor

prune:
	rm -rf /Users/vadim/.badgerdb && rm -rf /home/vadim/.badgerdb

prune-idea:
	rm -rf .local/share/JetBrains
	rm -rf .config/JetBrains
	rm -rf .cache/JetBrains
	rm -rf .java/.userPrefs
	rm -rf ~/.config/JetBrains
	rm -rf ~/.cache/JetBrains
	rm -rf ~/.local/share/JetBrains

check-heap:
	go build -gcflags="-m" main.go

docker-build:
	docker build -t warpnet .

compose-up:
	docker compose --parallel 1 up -d --quiet-pull --build

bootstrap-port:
	nc -zv bootstrap.warp.net 4001