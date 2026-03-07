FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/ apps/

WORKDIR /app/apps/gw-gateway
RUN go build -o /gw-gateway .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /gw-gateway /gw-gateway
EXPOSE 8080
ENTRYPOINT ["/gw-gateway"]
