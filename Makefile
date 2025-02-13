install-garble:
	go install mvdan.cc/garble@latest

obfuscate-build:
	garble build -literals -mod=vendor -v -o warpnet cmd/node/member/main.go

oapi-codegen-install:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

ggen:
	oapi-codegen -generate server,types,spec,skip-prune -package api ./spec/local-api.yml > server/api-gen/api.gen.go

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

setup-hooks:
	git config core.hooksPath .githooks

ssh-do:
	ssh root@207.154.221.44

self-sign:
	openssl genpkey -algorithm Ed25519 -out private.pem
	openssl pkey -in private.pem -pubout -out public.pem
	openssl pkeyutl -sign -inkey private.pem -rawin -in warpnet -out warpnet.sig

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -gcflags=all=-l -mod=vendor -v -o warpnet-win.exe cmd/node/member/main.go

build-macos:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -gcflags=all=-l -mod=vendor -v -o warpnet-darwin cmd/node/member/main.go