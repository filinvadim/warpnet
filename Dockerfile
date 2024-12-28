ARG NODE_ID

FROM golang:1.22 AS builder

COPY . /go/src/github.com/filinvadim/warpnet
WORKDIR /go/src/github.com/filinvadim/warpnet

RUN CGO_ENABLED=0 go build -v -mod vendor -o warpnet cmd/node/bootstrap/main.go

FROM alpine:3.20
COPY --from=builder /go/src/github.com/filinvadim/warpnet/warpnet /warpnet

# Устанавливаем переменную окружения
ARG NODE_ID
ENV NODE_ID=${NODE_ID}

EXPOSE 4001

# Используем shell для интерпретации переменных окружения
CMD sh -c "/warpnet --id ${NODE_ID}"
