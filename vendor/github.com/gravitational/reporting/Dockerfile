FROM quay.io/gravitational/debian-venti:go1.9.1-jessie

ARG PROTOC_VER
ARG GOGO_PROTO_TAG
ARG GRPC_GATEWAY_TAG
ARG PLATFORM

ENV TARBALL protoc-${PROTOC_VER}-${PLATFORM}.zip
ENV GOGOPROTO_ROOT ${GOPATH}/src/github.com/gogo/protobuf
ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    GOPATH="/gopath" \
    PATH="$PATH:/opt/protoc/bin:/opt/go/bin:/gopath/bin"

RUN apt-get update && apt-get install unzip

RUN curl -L -o /tmp/${TARBALL} https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/${TARBALL}
RUN cd /tmp && unzip /tmp/protoc-${PROTOC_VER}-linux-x86_64.zip -d /usr/local && rm /tmp/${TARBALL}

RUN go get -u github.com/gogo/protobuf/proto github.com/gogo/protobuf/protoc-gen-gogo github.com/gogo/protobuf/gogoproto golang.org/x/tools/cmd/goimports github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
RUN cd ${GOPATH}/src/github.com/gogo/protobuf && git reset --hard ${GOGO_PROTO_TAG} && make install
RUN cd ${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway && git reset --hard ${GRPC_GATEWAY_TAG} && go install ./protoc-gen-grpc-gateway

ENV PROTO_INCLUDE "/usr/local/include":"${GOPATH}/src":"${GOPATH}/src/github.com/gogo/protobuf/protobuf":"${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis":"${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis":"${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"
