FROM quay.io/gravitational/mkdocs-base:1.0.3

ARG UID
ARG GID
ARG USER
ARG WORKDIR

RUN apt-get update -y
RUN apt-get install -q -y ruby-sass parallel

RUN groupadd $USER --gid=$GID -o && useradd $USER --uid=$UID --gid=$GID --create-home --shell=/bin/bash
RUN echo "source /etc/profile" > /home/$USER/.bashrc

VOLUME ["$WORKDIR"]
EXPOSE 6600
