FROM golang:1.22-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /srv

RUN apk add --no-cache --update git bash curl tzdata && \
    cp /usr/share/zoneinfo/Asia/Almaty /etc/localtime && \
    rm -rf /var/cache/apk/*

COPY ./cmd /srv/cmd
COPY ./groxypb /srv/groxypb
COPY ./pkg /srv/pkg

COPY ./go.mod /srv/go.mod
COPY ./go.sum /srv/go.sum

COPY ./.git/ /srv/.git

RUN \
    export version="$(git describe --tags --long)" && \
    echo "version: $version" && \
    go build -o /go/build/groxy -ldflags "-X 'main.version=${version}' -s -w" /srv/cmd/groxy

FROM alpine:3.14 AS base

RUN apk add --no-cache --update tzdata && \
    cp /usr/share/zoneinfo/Asia/Almaty /etc/localtime && \
    rm -rf /var/cache/apk/*

FROM scratch
LABEL org.opencontainers.image.source="https://github.com/Semior001/groxy"
LABEL maintainer="Semior <ura2178@gmail.com>"

COPY --from=builder /go/build/groxy /usr/bin/groxy
COPY --from=base /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /etc/passwd /etc/passwd
COPY --from=base /etc/group /etc/group

WORKDIR /etc/groxy
ENTRYPOINT ["/usr/bin/groxy", "--file.name", "/etc/groxy/config.yaml"]
