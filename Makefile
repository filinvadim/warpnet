oapi-codegen-install:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

gen:
	~/go/bin/oapi-codegen -generate types,spec,skip-prune -package domain ./spec/domain.yml > domain-gen/domain.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/domain-gen -generate client,server,types,spec,skip-prune -package api ./spec/local-api.yml > interface/api-gen/api.gen.go
	~/go/bin/oapi-codegen -import-mapping ./domain.yml:github.com/filinvadim/warpnet/domain-gen -generate client,server,types,spec,skip-prune -package node ./spec/node.yml > node-gen/api.gen.go
tests:
	CGO_ENABLED=0 go test -count=1 -short ./...

govendor:
	go mod tidy
	go mod vendor

prune:
	rm -rf /Users/vadim/.badgerdb && rm -rf /home/vadim/.badgerdb

run:
	go run main.go

prune-idea:
	rm -rf .local/share/JetBrains
	rm -rf .config/JetBrains
	rm -rf .cache/JetBrains
	rm -rf .java/.userPrefs

check-heap:
	go build -gcflags="-m" main.go