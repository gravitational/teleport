FROM scratch

ARG ALPINE_VERSION

ADD alpine-minirootfs-$ALPINE_VERSION-aarch64.tar.gz /

COPY webserver /webserver

CMD [ "/webserver" ]
