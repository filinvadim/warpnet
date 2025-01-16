oapi-codegen-install:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

ggen:
	~/go/bin/oapi-codegen -generate types,spec,skip-prune -package domain ./spec/domain.yml > gen/domain-gen/domain.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/gen/domain-gen -generate types,spec,skip-prune -package dto ./spec/dto.yml > gen/dto-gen/dto.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/gen/domain-gen -generate types,spec,skip-prune -package event ./spec/event.yml > gen/event-gen/event.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/gen/domain-gen -generate server,types,spec,skip-prune -package api ./spec/local-api.yml > server/api-gen/api.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/gen/domain-gen -generate types,spec,skip-prune -package node ./spec/node.yml > core/node-gen/node.gen.go
tests:
	CGO_ENABLED=0 go test -count=1 -short ./...

prune:
	rm -rf /Users/vadim/.badgerdb && rm -rf /home/vadim/.badgerdb

check-heap:
	go build -gcflags="-m" main.go

docker-build:
	docker build -t warpnet .

compose-up:
	docker compose --parallel 1 up -d --quiet-pull --build

