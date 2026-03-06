FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copy go.work + workspace modules needed for the build
COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/report-service/ apps/report-service/

WORKDIR /app/apps/report-service
RUN go build -o /report-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /report-service /report-service
EXPOSE 50055
ENTRYPOINT ["/report-service"]
