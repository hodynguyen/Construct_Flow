FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/ apps/

WORKDIR /app/apps/file-service
RUN go build -o /file-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /file-service /file-service
EXPOSE 50054
ENTRYPOINT ["/file-service"]
