# Nebula Operator e2e testing

# e2e run default with kind

```shell
export E2E_DOCKER_CONFIG_JSON_SECRET=`cat ~/.docker/config.json| base64 -w 0`
export E2E_OPERATOR_IMAGE= # your own nebula-operator image
make e2e
```

# run with existing kubernetes

```shell
export E2E_DOCKER_CONFIG_JSON_SECRET=`cat ~/.docker/config.json| base64 -w 0`
export E2E_OPERATOR_INSTALL=false # if you already install nebula-operator
make e2e E2EARGS="--kubeconfig ~/.kube/config"
```

# run with enterprise edition

```shell
export E2E_DOCKER_CONFIG_JSON_SECRET=`cat ~/.docker/config.json| base64 -w 0`
export E2E_OPERATOR_INSTALL=false # if you already install nebula-operator
export E2E_NC_VERSION=v3.5.1 # if you want to specify version
export E2E_NC_GRAPHD_IMAGE=reg.vesoft-inc.com/vesoft-ent/nebula-graphd-ent
export E2E_NC_METAD_IMAGE=reg.vesoft-inc.com/vesoft-ent/nebula-metad-ent
export E2E_NC_STORAGED_IMAGE=reg.vesoft-inc.com/vesoft-ent/nebula-storaged-ent
export E2E_NC_LICENSE_MANAGER_URL=license.vesoft-inc.com:9119
make e2e E2EARGS="--kubeconfig ~/.kube/config"
```

# run certain specified cases

The test cases can be filtered using `-labels key1=value1,key2=value2` and `-feature regular-expression` flag.

```shell
# use -labels
make e2e E2EARGS="--kubeconfig ~/.kube/config -labels category=basic,group=scale"

# use -feature
make e2e E2EARGS="--kubeconfig ~/.kube/config -feature 'scale.*default'"

# use -labels and -feature
make e2e E2EARGS="--kubeconfig ~/.kube/config -labels category=basic,group=scale -feature 'scale.*default'"
```
