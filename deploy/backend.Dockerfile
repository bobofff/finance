FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/finance-backend ./cmd/server

FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /bin/finance-backend /app/finance-backend
RUN mkdir -p /app/logs /logs && chown -R app:app /app /logs
USER app

EXPOSE 8888
CMD ["/app/finance-backend"]
