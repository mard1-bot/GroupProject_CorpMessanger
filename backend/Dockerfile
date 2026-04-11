FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . ./

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/api ./cmd/api

FROM alpine:3.20

WORKDIR /app

RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /app/api /app/api

USER app

EXPOSE 8080

CMD ["/app/api"]