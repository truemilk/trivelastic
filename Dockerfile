FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o server -ldflags="-w -s"

FROM alpine:3.19
WORKDIR /app
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/server .
EXPOSE 8080
ENTRYPOINT ["/app/server"]
