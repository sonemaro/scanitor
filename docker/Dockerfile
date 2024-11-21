FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /scanitor cmd/scanitor/main.go

FROM alpine:latest
COPY --from=builder /scanitor /usr/local/bin/
ENTRYPOINT ["scanitor"]
