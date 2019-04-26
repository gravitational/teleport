FROM golang:1.12.4-stretch

ARG PROTOC_VER
ARG GOGO_PROTO_TAG
ARG PLATFORM

ENV TARBALL protoc-${PROTOC_VER}-${PLATFORM}.zip
ENV GOGOPROTO_ROOT ${GOPATH}/src/github.com/gogo/protobuf

RUN apt-get update && apt-get install unzip

RUN curl -L -o /tmp/${TARBALL} https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/${TARBALL}
RUN cd /tmp && unzip /tmp/protoc-${PROTOC_VER}-linux-x86_64.zip -d /usr/local && rm /tmp/${TARBALL}

RUN go get -u github.com/gogo/protobuf/proto github.com/gogo/protobuf/protoc-gen-gogo github.com/gogo/protobuf/gogoproto golang.org/x/tools/cmd/goimports
RUN cd ${GOPATH}/src/github.com/gogo/protobuf && git reset --hard ${GOGO_PROTO_TAG} && make install

ENV PROTO_INCLUDE "/usr/local/include":"${GOPATH}/src":"${GOPATH}/src/github.com/gogo/protobuf/protobuf":"${GOGOPROTO_ROOT}":"${GOGOPROTO_ROOT}/protobuf":"${GOPATH}/src/github.com/gravitational/teleport/lib/services"

