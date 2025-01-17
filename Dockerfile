FROM golang:1.23 AS builder

COPY . /go/src/github.com/filinvadim/warpnet
WORKDIR /go/src/github.com/filinvadim/warpnet

ENV GOPROXY=''
ENV GOSUMDB=''
ENV GOPRIVATE='github.com/filinvadim/warpnet'
ENV GO111MODULE=''
ENV CGO_ENABLED=0

RUN  go build -mod=vendor -v -o warpnet cmd/node/bootstrap/main.go

FROM alpine:3.20
COPY --from=builder /go/src/github.com/filinvadim/warpnet/warpnet /warpnet

ENV SEED_ID="1"

EXPOSE 4001 4002

CMD ["/warpnet"]
