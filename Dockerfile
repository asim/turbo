FROM golang:1.20-alpine as builder
RUN apk --no-cache add gcc g++ libtool musl-dev
WORKDIR /app
COPY . .
RUN go mod download
RUN export CGO_ENABLED=1; export CC=gcc; go build -ldflags="-linkmode=external -s -w" -o turbo main.go
RUN export CGO_ENABLED=1; export CC=gcc; go build -ldflags="-linkmode=external -s -w" -o admin cmd/admin/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates && rm -rf /var/cache/apk/* /tmp/* 
WORKDIR /app
COPY --from=builder /app/turbo .
COPY --from=builder /app/admin .
ENTRYPOINT ["./turbo"]
