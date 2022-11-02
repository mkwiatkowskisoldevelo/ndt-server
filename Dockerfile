FROM golang:1.18-alpine3.14 as ndt-server-build
RUN apk add --no-cache git gcc linux-headers musl-dev
ADD . /go/src/github.com/m-lab/ndt-server
RUN /go/src/github.com/m-lab/ndt-server/build.sh

# Now copy the built image into the minimal base image
FROM alpine:3.14
COPY --from=ndt-server-build /go/bin/ndt-server /
ADD ./html /html
WORKDIR /

ARG USER=rootless
ARG UID=1000
ARG GID=1001

RUN addgroup --system --gid $GID rootless_users
RUN adduser -D -u $UID -g $GID $USER

RUN mkdir ndtdata

RUN chown -R $USER:users /ndt-server \
    /ndtdata \
    /html

USER $USER:users

ENTRYPOINT ["/ndt-server"]
