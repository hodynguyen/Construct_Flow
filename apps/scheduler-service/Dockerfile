FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/scheduler-service/ apps/scheduler-service/

WORKDIR /app/apps/scheduler-service
RUN go build -o /scheduler-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /scheduler-service /scheduler-service
EXPOSE 50058
ENTRYPOINT ["/scheduler-service"]
