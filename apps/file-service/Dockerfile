FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copy go.work + workspace modules needed for the build
COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/file-service/ apps/file-service/

WORKDIR /app/apps/file-service
RUN go build -o /file-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /file-service /file-service
EXPOSE 50054
ENTRYPOINT ["/file-service"]
