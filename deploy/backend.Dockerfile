FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/finance-backend ./cmd/server

FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
USER app
WORKDIR /app
COPY --from=builder /bin/finance-backend /app/finance-backend

EXPOSE 8888
CMD ["/app/finance-backend"]
