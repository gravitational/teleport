FROM quay.io/gravitational/mkdocs-base:0.16.1

ARG UID
ARG GID
ARG USER
ARG WORKDIR

RUN apt-get install -q -y ruby-sass

RUN groupadd $USER --gid=$GID -o && useradd $USER --uid=$UID --gid=$GID --create-home --shell=/bin/bash
RUN echo "source /etc/profile" > /home/$USER/.bashrc

VOLUME ["$WORKDIR"]
EXPOSE 6600
