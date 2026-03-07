FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/ apps/

WORKDIR /app/apps/audit-service
RUN go build -o /audit-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /audit-service /audit-service
EXPOSE 50056
ENTRYPOINT ["/audit-service"]
