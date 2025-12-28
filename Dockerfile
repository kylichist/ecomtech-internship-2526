FROM golang:1.25.5 AS builder

WORKDIR /build

COPY go.mod server.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM alpine:latest AS final

WORKDIR /app

RUN apk --no-cache add curl # для healthcheck

COPY --from=builder /build/server .

EXPOSE 8080

CMD ["./server"]