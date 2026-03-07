FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY gen/go/ gen/go/
COPY apps/ apps/

WORKDIR /app/apps/user-service
RUN go build -o /user-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /user-service /user-service
EXPOSE 50053
ENTRYPOINT ["/user-service"]
