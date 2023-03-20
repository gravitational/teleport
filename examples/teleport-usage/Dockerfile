FROM golang:1.20-bullseye
WORKDIR /app
COPY go.mod go.sum main.go /app/
RUN go build -o teleport_usage
ENTRYPOINT ["./teleport_usage"]
