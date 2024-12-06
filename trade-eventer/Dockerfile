FROM golang:1.21-alpine

WORKDIR /app

RUN mkdir -p /app/shared /app/trade-eventer
COPY shared/ shared

WORKDIR /app/trade-eventer

COPY trade-eventer/go.mod .
RUN go mod download

COPY trade-eventer/ .

RUN go build -o trade-eventer .

CMD ["./trade-eventer"]
