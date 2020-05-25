#FROM golang AS builder
#MAINTAINER "Cuong Manh Le <cuong.manhle.vn@gmail.com>"
#
#RUN apt-get update && \
#    apt-get install -y --no-install-recommends build-essential && \
#    apt-get clean && \
#    mkdir -p "$GOPATH/src/kubewatch"
#
#ADD . "$GOPATH/src/kubewatch"
#
#RUN cd "$GOPATH/src/kubewatch" && \
#    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a --installsuffix cgo --ldflags="-s" -o /kubewatch
#
#FROM bitnami/minideb:stretch
#RUN install_packages ca-certificates
#
#COPY --from=builder /kubewatch /bin/kubewatch
#
#ENTRYPOINT ["/bin/kubewatch"]
#
# FROM 10.70.24.123:7999/library/golang:1.13.2-alpine3.10 as builder
FROM golang:1.13.11-alpine3.11 as builder

WORKDIR /kw

COPY go.mod .
COPY go.sum .
COPY . .
COPY upx-3.95-amd64_linux.tar.xz /usr/local
RUN go env -w GOPROXY=https://goproxy.cn,direct && go env -w GO111MODULE=on && go mod download
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && \
    apk add --no-cache xz && xz -d -c /usr/local/upx-3.95-amd64_linux.tar.xz | tar -xOf - upx-3.95-amd64_linux/upx > /usr/bin/upx && \
    chmod a+x /usr/bin/upx && go build -ldflags '-w -s' -o kubewatch && upx kubewatch

# FROM 10.70.24.123:7999/library/alpine:latest
FROM alpine:latest

COPY --from=builder /kw/kubewatch /usr/local/bin/

EXPOSE 5000
CMD ["/usr/local/bin/kubewatch"]