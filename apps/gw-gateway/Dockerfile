FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/gw-gateway/ apps/gw-gateway/

WORKDIR /app/apps/gw-gateway
RUN go build -o /gw-gateway .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /gw-gateway /gw-gateway
# Casbin policy files
COPY apps/gw-gateway/casbin/ /casbin/
EXPOSE 8080
ENTRYPOINT ["/gw-gateway"]
