FROM golang:1.24 AS builder

COPY . /warpnet
WORKDIR /warpnet

ENV GOPROXY=''
ENV GOSUMDB=''
ENV GOPRIVATE='github.com/filinvadim/warpnet'
ENV GO111MODULE=''
ENV CGO_ENABLED=0

RUN go build -ldflags "-s -w" -gcflags=all=-l -mod=vendor -v -o warpnet cmd/node/bootstrap/main.go
EXPOSE 4001 4002

VOLUME /tmp/snapshot

CMD ["/warpnet/warpnet"]
