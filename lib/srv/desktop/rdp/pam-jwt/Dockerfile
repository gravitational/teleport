FROM rust:1.88 AS builder

WORKDIR /usr/src
COPY . .

RUN apt-get update && apt-get install -y libpam0g libpam0g-dev

RUN cargo build

FROM ubuntu

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y xfce4 xvfb fuse libpam0g libfuse2 dbus-x11 xrdp software-properties-common pamtester nano mousepad

COPY mozilla /etc/apt/preferences.d/mozilla

RUN add-apt-repository ppa:mozillateam/ppa && apt-get update && apt-get install -y --allow-downgrades firefox

ENV RUST_BACKTRACE=full

RUN useradd -ms /bin/bash newuser && echo "newuser:pass"|chpasswd && mkdir "/tmp/.X11-unix/"

COPY --from=builder /usr/src/target/debug/libpam_jwt.so /lib/aarch64-linux-gnu/security/pam_jwt.so
COPY xrpd.pamd /etc/pam.d/xrdp-sesman
COPY run_rdp.sh /opt/run_rdp.sh

RUN chmod +x /opt/run_rdp.sh

CMD /opt/run_rdp.sh