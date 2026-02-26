FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o headlamp-auth-proxy .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/headlamp-auth-proxy /usr/local/bin/
USER 65534:65534
ENTRYPOINT ["headlamp-auth-proxy"]
