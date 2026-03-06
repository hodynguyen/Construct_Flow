FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copy go.work + workspace modules needed for the build
COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/task-service/ apps/task-service/

WORKDIR /app/apps/task-service
RUN go build -o /task-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /task-service /task-service
EXPOSE 50051
ENTRYPOINT ["/task-service"]
