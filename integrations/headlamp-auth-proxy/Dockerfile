FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o headlamp-auth-proxy .

ARG BASE_IMAGE=gcr.io/distroless/static-debian12
FROM $BASE_IMAGE
COPY --from=builder /app/headlamp-auth-proxy /
ENTRYPOINT ["/headlamp-auth-proxy"]
