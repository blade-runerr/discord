FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /mini-discord ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /mini-discord /usr/local/bin/mini-discord
COPY web ./web

RUN mkdir -p /app/data && chown -R app:app /app

ENV PORT=8000
EXPOSE 8000

USER app
CMD ["mini-discord"]
