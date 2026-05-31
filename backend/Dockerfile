FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY migrations ./migrations/

ENV GOPROXY=https://proxy.golang.org,https://goproxy.cn,direct
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/api ./cmd/api

FROM alpine:3.20

WORKDIR /app

RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /app/api /app/api
COPY --from=builder /app/migrations /app/migrations

# Create uploads directory and set permissions for the app user
RUN mkdir -p /app/uploads && chown -R app:app /app/uploads

USER app

EXPOSE 8080

CMD ["/app/api"]