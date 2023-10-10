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
