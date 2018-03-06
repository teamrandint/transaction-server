# build stage
FROM golang:alpine AS build-env
COPY . /go/src/seng468/transaction-server
RUN apk add --no-cache git \
    && go get github.com/garyburd/redigo/redis \
    && go get github.com/patrickmn/go-cache \
    && go get github.com/shopspring/decimal \
    && cd /go/src/seng468/transaction-server \
    && go build -o transactionserve

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/seng468/transaction-server/transactionserve /app/
EXPOSE 44455-44459
ENTRYPOINT ./transactionserve 