second-member:
	go run cmd/node/member/main.go --database.dir storage2 --node.port 4021 --server.port 4022

third-member:
	go run cmd/node/member/main.go --database.dir storage3 --node.port 4031 --server.port 4032

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

update-deps:
	GOPRIVATE=github.com/filinvadim/warpnet-frontend go get -v -u all
	go mod vendor

get-frontend:
	rm -rf vendor/github.com/filinvadim/warpnet-frontend/release
	GOPRIVATE=github.com/filinvadim/warpnet-frontend go get github.com/filinvadim/warpnet-frontend
	go mod vendor

setup-hooks:
	git config core.hooksPath .githooks

ssh-do:
	ssh root@207.154.221.44
