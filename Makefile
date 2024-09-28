oapi-codegen-install:
	go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.2.0
gen:
	~/go/bin/oapi-codegen -generate types,spec,skip-prune -package components ./spec/components.yml > api/components/components.gen.go
	~/go/bin/oapi-codegen -import-mapping ./components.yml:github.com/filinvadim/dWighter/api/components -generate server,types,skip-prune -package api ./spec/api.yml > api/api/api.gen.go
	~/go/bin/oapi-codegen -import-mapping ./components.yml:github.com/filinvadim/dWighter/api/components -generate client,server,types,skip-prune -package discovery ./spec/discovery.yml > api/discovery/discovery.gen.go

tests:
	CGO_ENABLED=0 go test -count=1 -short ./...

govendor:
	go mod tidy
	go mod vendor

prune:
	rm -rf /Users/vadim/.badgerdb
	rm -rf /home/vadim/.badgerdb

run:
	go run main.go