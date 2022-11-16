# syntax=docker/dockerfile:1

FROM --platform=${BUILDPLATFORM} golang:1.19 as build
WORKDIR /wd

COPY go.mod go.sum .
RUN go mod download

COPY . .

ARG TARGETOS TARGETARCH
RUN make GOOS=${TARGETOS} GOARCH=${TARGETARCH} prehog-verbose

FROM gcr.io/distroless/static-debian11:nonroot
COPY --from=build /wd/build/prehog /prehog
ENTRYPOINT [ "/prehog" ]
