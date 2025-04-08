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

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -gcflags=all=-l -mod=vendor -v -o warpnet-win.exe cmd/node/member/main.go

build-macos:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -gcflags=all=-l -mod=vendor -v -o release/darwin/warpnet.app/Contents/MacOS/warpnet-darwin cmd/node/member/main.go
	chmod +x release/darwin/warpnet.app/Contents/MacOS/warpnet-darwin
	chmod +x release/darwin/warpnet.app/Contents/MacOS/launcher

build-linux:
	#sudo dpkg -P warpnet
	rm -f release/linux/warpnet.deb
	go build -ldflags "-s -w" -gcflags=all=-l -mod=vendor -v -o release/linux/warpnet/usr/local/bin/warpnet cmd/node/member/main.go
	chmod +x release/linux/warpnet/usr/local/bin/warpnet
	dpkg-deb --build release/linux/warpnet release/linux/warpnet.deb
	desktop-file-validate release/linux/warpnet/usr/share/applications/warpnet.desktop
