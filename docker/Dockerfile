FROM busybox:latest
ARG UID
ARG UNAME
ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    PATH="$PATH:/opt/teleport"
RUN adduser $UNAME -u $UID -S -s /bin/sh
EXPOSE 3080 3025 3024 3022 3023
