oapi-codegen-install:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

ggen:
	~/go/bin/oapi-codegen -generate types,spec,skip-prune -package domain ./spec/domain.yml > gen/domain-gen/domain.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/gen/domain-gen -generate types,spec,skip-prune -package event ./spec/event.yml > gen/event-gen/event.gen.go
	~/go/bin/oapi-codegen -import-mapping ./event.yml:github.com/filinvadim/warpnet/gen/event-gen -generate server,types,spec,skip-prune -package api ./spec/local-api.yml > gen/api-gen/api.gen.go
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

gosec:
	~/go/bin/gosec ./...

get-frontend:
	rm -rf vendor/github.com/filinvadim/warpnet-frontend/dist
	GOPRIVATE=github.com/filinvadim/warpnet-frontend go get github.com/filinvadim/warpnet-frontend
	go mod vendor

ssh:
	ssh root@207.154.221.44

gen-md:
	./embedmd -w README.md

self-sign:
	openssl genpkey -algorithm Ed25519 -out private.pem
	openssl pkey -in private.pem -pubout -out public.pem
	openssl pkeyutl -sign -inkey private.pem -rawin -in warpnet -out warpnet.sig