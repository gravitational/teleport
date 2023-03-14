ARG TELEPORT_TAG
FROM public.ecr.aws/gravitational/teleport:${TELEPORT_TAG}

# Demo ansible, ssh, htop
RUN apt-get update && apt-get install -y ansible ssh inetutils-syslogd htop

RUN mkdir /run/sshd

VOLUME ["/teleport", "/var/lib/teleport"]

COPY ./etc/teleport.yaml /etc/teleport.d/teleport.yaml
COPY ./.bashrc /root/.bashrc
COPY ./.screenrc /root/.screenrc
COPY ./scripts /etc/teleport.d/scripts
COPY ./ansible /etc/teleport.d/ansible
COPY ./pam.d/ssh /etc/pam.d/ssh
COPY ./etc/ssh/sshd_config /etc/ssh/sshd_config
