FROM golang:latest
LABEL maintainer="Mike Lloyd <mike@reboot3times.org>"

RUN go get -v -d github.com/allegro/bigcache && \
    go build -v -o bigcache github.com/allegro/bigcache/server && \
    mv bigcache /usr/bin/bigcache

EXPOSE 9090

CMD ["bigcache", "-NoSecurityForMe", "-port=9090"]
