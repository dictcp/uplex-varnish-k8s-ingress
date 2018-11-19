FROM centos:centos7

COPY varnishcache_varnish60.repo /etc/yum.repos.d/

RUN yum install -y epel-release && yum update -y -q && \
    yum -q makecache -y --disablerepo='*' --enablerepo='varnishcache_varnish60' && \
    yum-config-manager --add-repo https://pkg.uplex.de/rpm/7/uplex-varnish/x86_64/ && \
    yum install -y -q varnish-6.0.1 && \
    yum install -y -q --nogpgcheck vmod-re2-1.5.1 && \
    yum clean all && rm -rf /var/cache/yum

COPY k8s-ingress varnish/vcl/vcl.tmpl /

ENTRYPOINT ["/k8s-ingress"]
