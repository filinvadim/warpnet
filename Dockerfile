ARG SEED_ID

FROM golang:1.22 AS builder

COPY . /go/src/github.com/filinvadim/warpnet
WORKDIR /go/src/github.com/filinvadim/warpnet

RUN CGO_ENABLED=0 go build -mod=vendor -v -o warpnet cmd/node/bootstrap/main.go

FROM alpine:3.20
COPY --from=builder /go/src/github.com/filinvadim/warpnet/warpnet /warpnet

ENV SEED_ID=$SEED_ID

EXPOSE 4001 4002

CMD /warpnet
