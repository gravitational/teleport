FROM scratch
ADD ca-certificates.crt /etc/ssl/certs/
ADD build/main /
EXPOSE 9000
CMD ["/main", "-backend", "mem"]
