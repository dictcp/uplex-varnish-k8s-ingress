FROM centos:centos7

RUN yum install -y epel-release

RUN yum update -y -q

COPY varnishcache_varnish60.repo /etc/yum.repos.d/

RUN yum -q makecache -y --disablerepo='*' --enablerepo='varnishcache_varnish60'

RUN yum-config-manager --add-repo https://pkg.uplex.de/rpm/7/uplex-varnish/x86_64/

RUN yum install -y -q varnish-6.0.1

RUN yum install -y -q --nogpgcheck vmod-re2

COPY k8s-ingress varnish/vcl/vcl.tmpl /

ENTRYPOINT ["/k8s-ingress"]
