# The base image (buildbox:latest) is built by running `make -C build.assets` 
# from the base repo directory $GOPATH/gravitational.com/teleport
FROM teleport-buildbox:latest

# DEBUG=1 is needed for the Web UI to be loaded from static assets instead 
# of the binary
ENV DEBUG=1

# htop is useful for testing terminal resizing
RUN apt-get install -y htop

VOLUME ["/teleport", "/var/lib/teleport"]
COPY .bashrc /root/.bashrc

# expose only proxy ports (SSH and HTTPS)
EXPOSE 3023 3080
