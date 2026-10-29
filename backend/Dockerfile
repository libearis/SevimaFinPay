FROM golang:1.23-alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/mockgateway ./cmd/mockgateway

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/mockgateway /app/mockgateway

EXPOSE 8080

CMD ["/app/api"]
