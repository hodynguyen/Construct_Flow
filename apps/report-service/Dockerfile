FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/ apps/

WORKDIR /app/apps/report-service
RUN go build -o /report-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /report-service /report-service
EXPOSE 50055
ENTRYPOINT ["/report-service"]
