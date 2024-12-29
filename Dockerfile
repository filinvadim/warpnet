FROM golang:1.22 AS builder

COPY . /go/src/github.com/filinvadim/warpnet
WORKDIR /go/src/github.com/filinvadim/warpnet

RUN go mod vendor && go mod verify
RUN CGO_ENABLED=0 go build -v -o warpnet cmd/node/bootstrap/main.go

FROM alpine:3.20
COPY --from=builder /go/src/github.com/filinvadim/warpnet/warpnet /warpnet

EXPOSE 4001

CMD /warpnet
