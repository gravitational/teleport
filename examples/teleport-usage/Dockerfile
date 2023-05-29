FROM golang:1.20-bullseye
WORKDIR /app
COPY go.mod go.sum main.go /app/
RUN go build -o teleport_usage
FROM debian:stable-slim
RUN apt-get -y update && \
   apt-get install -y ca-certificates && \
   apt-get -y clean && \
   rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=0 /app/teleport_usage /app/
ENTRYPOINT ["./teleport_usage"]
