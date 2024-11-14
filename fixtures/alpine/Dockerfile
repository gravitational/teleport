FROM scratch

ARG ALPINE_VERSION

ADD alpine-minirootfs-$ALPINE_VERSION-x86_64.tar.gz /

COPY webserver /webserver

CMD [ "/webserver" ]
