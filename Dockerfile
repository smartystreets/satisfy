FROM scratch
ADD https://curl.haxx.se/ca/cacert.pem /etc/ssl/certs/ca-certificates.crt
ADD ./workspace/satisfy /satisfy
CMD ["/satisfy"]