FROM golang:1.10.3 as builder
COPY . /go/src/code.uplex.de/uplex-varnish/k8s-ingress
ARG PACKAGES
RUN go get -d -v github.com/slimhazard/gogitversion && \
    cd /go/src/github.com/slimhazard/gogitversion && \
    make install && \
    cd /go/src/code.uplex.de/uplex-varnish/k8s-ingress && \
    go get -v $PACKAGES && \
    go generate && \
    go build -o k8-ingress *.go

FROM centos:centos7
COPY varnishcache_varnish60.repo /etc/yum.repos.d/
RUN yum install -y epel-release && yum update -y -q && \
    yum -q makecache -y --disablerepo='*' --enablerepo='varnishcache_varnish60' && \
    yum-config-manager --add-repo https://pkg.uplex.de/rpm/7/uplex-varnish/x86_64/ && \
    yum install -y -q varnish-6.0.1 && \
    yum install -y -q --nogpgcheck vmod-re2-1.5.1 && \
    yum clean all && rm -rf /var/cache/yum
COPY varnish/vcl/vcl.tmpl /
COPY --from=builder /go/src/code.uplex.de/uplex-varnish/k8s-ingress .
ENTRYPOINT ["./k8s-ingress"]
