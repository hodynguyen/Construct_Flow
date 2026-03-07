FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/ apps/

WORKDIR /app/apps/notification-service
RUN go build -o /notification-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /notification-service /notification-service
EXPOSE 50052
ENTRYPOINT ["/notification-service"]
