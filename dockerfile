# ---------- STAGE 1: build ----------
FROM golang:1.22-alpine AS builder

# Instala dependências básicas
RUN apk add --no-cache git

WORKDIR /app

# Copia go.mod e go.sum primeiro (cache de dependências)
COPY go.mod go.sum ./
RUN go mod tidy

# Copia o restante do código
COPY . .

# Build da aplicação (binário estático)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server main.go

# ---------- STAGE 2: runtime ----------
FROM alpine:latest

WORKDIR /app

# Copia apenas o binário
COPY --from=builder /app/server .

# Porta exposta
EXPOSE 8080

# Comando de execução
CMD ["./server"]