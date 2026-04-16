FROM golang:1.25-alpine AS builder

WORKDIR /app

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь код
COPY . .

# Собираем
RUN CGO_ENABLED=0 GOOS=linux go build -o /news-parser ./cmd/parser

# Финальный минимальный образ
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/
COPY --from=builder /news-parser .

CMD ["./news-parser"]