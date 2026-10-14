FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /quicknotes .

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /quicknotes /usr/local/bin/quicknotes

RUN addgroup -S appgroup \
    && adduser -S appuser -G appgroup \
    && chown appuser:appgroup /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/quicknotes"]