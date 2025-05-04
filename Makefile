second-member:
	go run cmd/node/member/main.go --database.dir storage2 --node.port 4021 --server.port 4022

third-member:
	go run cmd/node/member/main.go --database.dir storage3 --node.port 4031 --server.port 4032

tests:
	CGO_ENABLED=0 go test -count=1 -short ./...

prune:
	- rm -rf $(HOME)/.badgerdb

check-heap:
	go build -gcflags="-m" main.go

update-deps:
	GOPRIVATE=github.com/filinvadim/warpnet-frontend go get -v -u all && go mod vendor

get-frontend:
	GOPRIVATE=github.com/filinvadim/warpnet-frontend go get github.com/filinvadim/warpnet-frontend && go mod vendor

setup-hooks:
	git config core.hooksPath .githooks

ssh-do:
	ssh root@207.154.221.44

build-macos:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -gcflags=all=-l -mod=vendor -v -o warpnet-darwin cmd/node/member/main.go
	chmod +x warpnet-darwin