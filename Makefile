oapi-codegen-install:
	go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.2.0
gen:
	~/go/bin/oapi-codegen -generate server,types,spec,skip-prune -package server ./spec/api.yml > api/server/server.gen.go
	~/go/bin/oapi-codegen -generate client,types,spec,skip-prune -package client ./spec/api.yml > api/client/client.gen.go

tests:
	CGO_ENABLED=0 go test -count=1 -short ./...

govendor:
	go mod tidy
	go mod vendor

prune:
	rm -rf /Users/vadim/.badgerdb

run:
	go run main.go