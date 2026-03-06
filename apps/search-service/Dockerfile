FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/search-service/ apps/search-service/

WORKDIR /app/apps/search-service
RUN go build -o /search-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /search-service /search-service
EXPOSE 50057
ENTRYPOINT ["/search-service"]
