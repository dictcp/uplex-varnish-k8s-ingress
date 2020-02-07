#! /bin/bash -ex

function undeploy_and_clear {
    cd ${MYPATH}/../deploy
    ./undeploy.sh
    ./clear.sh
}

MYPATH="$( cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 ; pwd -P )"
trap undeploy_and_clear EXIT

export TESTOPTS=-v

echo Test initial deployment, and Varnish Ingress deployment
cd ${MYPATH}/../deploy/
./init.sh
./deploy.sh
./verify.sh

echo "Hello, world!" example
cd ${MYPATH}/../examples/hello/
./deploy.sh
./verify.sh
./undeploy.sh

echo Single namespace example
cd ${MYPATH}/../examples/namespace/
./deploy.sh
./verify.sh
./undeploy.sh

echo Varnish Pod template with CLI args example
cd ${MYPATH}/../examples/varnish_pod_template/
./deploy_cli-args.sh
./verify_cli-args.sh

echo Varnish Pod template with PROXY protocol example
./deploy_proxy.sh
./verify_proxy.sh

echo Varnish Pod template with env settings example
./deploy_env.sh
./verify_env.sh

./undeploy.sh

echo Cluster-wide Ingress example
cd ${MYPATH}/../examples/architectures/clusterwide/
./deploy.sh
./verify.sh
./undeploy.sh

echo Example with a cluster-wide Ingress and a namespace-specific Ingress
cd ${MYPATH}/../examples/architectures/cluster-and-ns-wide/
./deploy.sh
./verify.sh
./undeploy.sh

echo Multiple Varnish Services in a namespace example
cd ${MYPATH}/../examples/architectures/multi-varnish-ns/
./deploy.sh
./verify.sh
./undeploy.sh

echo Multiple Ingress controllers example
cd ${MYPATH}/../examples/architectures/multi-controller/
./deploy.sh
./verify.sh
./undeploy.sh

echo Custom VCL example
cd ${MYPATH}/../examples/custom-vcl/
./deploy.sh
./verify.sh
./undeploy.sh

echo Self-sharding cluster example
cd ${MYPATH}/../examples/self-sharding/
./deploy.sh
./verify.sh
./undeploy.sh

echo Basic Authentication example
cd ${MYPATH}/../examples/authentication/
./deploy_basic_auth.sh
./verify_basic_auth.sh
./undeploy_basic_auth.sh

echo Proxy Authentication example
./deploy_proxy_auth.sh
./verify_proxy_auth.sh
./undeploy_proxy_auth.sh

echo Combined ACL and Basic Authentication example
./deploy_acl_or_auth.sh
./verify_acl_or_auth.sh
./undeploy_acl_or_auth.sh

echo Access control list examples
cd ${MYPATH}/../examples/acl/
./deploy.sh
./verify.sh
./undeploy.sh

echo Rewrite rule examples
cd ${MYPATH}/../examples/rewrite/
./deploy.sh
./verify.sh
./undeploy.sh

echo Request disposition examples: re-implementing default vcl_recv
cd ${MYPATH}/../examples/req-disposition/
./deploy_builtin.sh
./verify_builtin.sh
./undeploy_builtin.sh

echo Request disposition examples: alternative re-implemention of default vcl_recv
./deploy_alt-builtin.sh
./verify_alt-builtin.sh
./undeploy_alt-builtin.sh

echo Request disposition examples: pass on certain cookies, lookup on all others
./deploy_pass-on-session-cookie.sh
./verify_pass-on-session-cookie.sh
./undeploy_pass-on-session-cookie.sh

echo Request disposition examples: cacheability rules by URL path pattern
./deploy_cacheability.sh
./verify_cacheability.sh
./undeploy_cacheability.sh

echo Request disposition examples: URL white- and blacklisting
./deploy_url-whitelist.sh
./verify_url-whitelist.sh
./undeploy_url-whitelist.sh

echo Request disposition examples: PURGE method
./deploy_purge.sh
./verify_purge.sh
./undeploy_purge.sh

echo BackendConfig example
cd ${MYPATH}/../examples/backend-config/
./deploy.sh
./verify.sh
./undeploy.sh

exit 0
