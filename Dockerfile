FROM golang:1.22 AS builder

COPY . /go/src/github.com/filinvadim/warpnet
WORKDIR /go/src/github.com/filinvadim/warpnet

RUN CGO_ENABLED=0 go build -v -mod vendor -o warpnet main.go

FROM alpine:3.20
COPY --from=builder /go/src/github.com/filinvadim/warpnet/warpnet /warpnet
CMD ["/warpnet", "--is-bootstrap"]