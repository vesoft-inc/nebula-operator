#!/bin/env bash

# Copyright 2024 Vesoft Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

if [ -n "$DEBUG" ]; then
	set -x
fi

set -o errexit
set -o nounset
set -o pipefail

# Turn off all attributes
NORM=`tput sgr0`
# Set bold mode
BOLD=`tput bold`


log() {
  echo \["$(date +%Y/%m/%d-%H:%M:%S)"\] "$1"
}

cmd_check() {
  if command -v jq > /dev/null 2>&1; then
    log "[cmd_check] jq exists"
  else
    log "[cmd_check] jq does not exist"
    exit 1
  fi

  if command -v kubectl > /dev/null 2>&1; then
    log "[cmd_check] kubectl exists"
  else
    log "[cmd_check] kubectl does not exist"
    exit 1
  fi
}

#########################
# HELP
#########################

help() {
  echo "This script restarts nebula cluster in Kubernetes"
  echo ""
  echo "Options:"
  echo "    -n      nebula cluster name"
  echo "    -N      nebula cluster namespace"
  echo "    -c      nebula component, valid value: graph, meta, storage, all"
  echo "    -r      retry interval for health check, default: 5"
  echo "    -t      graceful terminating period, default: 30"

  echo "    -h      view default help content"
}

log "Begin execution of nebula cluster restarting script on ${HOSTNAME}"
START_TIME=$SECONDS

#########################
# Parameter handling
#########################
NEBULA_CLUSTER_NAMESPACE=""
NEBULA_CLUSTER_NAME=""
NEBULA_COMPONENT=""
RETRY_INTERVAL=5
GRACEFUL_TERMINATING_PERIOD=20

#Loop through options passed
while getopts :n:N:c:p:i:h optname; do
  log "Option ${optname} set"
  case $optname in
    n) # set nebula cluster name
      NEBULA_CLUSTER_NAME="${OPTARG}"
      ;;
    N) # set nebula cluster namespace
      NEBULA_CLUSTER_NAMESPACE="${OPTARG}"
      ;;
    c) # set nebula component
      NEBULA_COMPONENT="${OPTARG}"
      ;;
    r) # set retry interval
      RESTART_INTERVAL="${OPTARG}"
      ;;
    t) # set terminating period
      GRACEFUL_TERMINATING_PERIOD="${OPTARG}"
      ;;
    h) #show help
      help
      exit 2
      ;;
    \?) #unrecognized option - show help
      echo -e \\n"Option -${BOLD}$OPTARG${NORM} not allowed."
      help
      exit 2
      ;;
  esac
done

#########################
# Functions
#########################
GRAPH_PATCH_FILE=graph_patch.json
META_PATCH_FILE=meta_patch.json
STORAGE_PATCH_FILE=storage_patch.json

restart_nebula() {
  case $NEBULA_COMPONENT in
    "graph")
      restart_graph
      ;;
    "meta")
      restart_meta
      ;;
    "storage")
      restart_storage
      ;;
    "all")
      restart_meta
      restart_storage
      restart_graph
      ;;
  esac
}

restart_graph() {
  log "[restart_graph] restart graph service"
  local workload="${NEBULA_CLUSTER_NAME}-graphd"
  local replicas=$(get_replicas ${workload} ${NEBULA_CLUSTER_NAMESPACE})
  log "[restart_graph] workload ${workload} replicas is $replicas"
  kubectl rollout restart sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE}
  local partition=$((replicas-1))
  while [ ${partition} -ge 0 ]; do
    echo {\"spec\": {\"updateStrategy\": {\"rollingUpdate\": {\"partition\": ${partition}}}}} | jq > ${GRAPH_PATCH_FILE}
    kubectl patch sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE} --patch-file ${GRAPH_PATCH_FILE}
    sleep ${GRACEFUL_TERMINATING_PERIOD}
    pod_name=${workload}-${partition}
    health_check ${pod_name} ${NEBULA_CLUSTER_NAMESPACE} graph
    partition=$((partition-1))
  done
  echo {\"spec\": {\"updateStrategy\": {\"rollingUpdate\": {\"partition\": ${replicas}}}}} | jq > ${GRAPH_PATCH_FILE}
  kubectl patch sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE} --patch-file ${GRAPH_PATCH_FILE}
  log "[restart_graph] workload ${workload} restart done!"
}

restart_meta() {
  log "[restart_meta] restart meta service"
  local workload="${NEBULA_CLUSTER_NAME}-metad"
  local replicas=$(get_replicas ${workload} ${NEBULA_CLUSTER_NAMESPACE})
  log "[restart_meta] workload ${workload} replicas is $replicas"
  kubectl rollout restart sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE}
  local partition=$((replicas-1))
  while [ ${partition} -ge 0 ]; do
    echo {\"spec\": {\"updateStrategy\": {\"rollingUpdate\": {\"partition\": ${partition}}}}} | jq > ${META_PATCH_FILE}
    kubectl patch sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE} --patch-file ${META_PATCH_FILE}
    sleep ${GRACEFUL_TERMINATING_PERIOD}
    pod_name=${workload}-${partition}
    health_check ${pod_name} ${NEBULA_CLUSTER_NAMESPACE} meta
    partition=$((partition-1))
  done
  echo {\"spec\": {\"updateStrategy\": {\"rollingUpdate\": {\"partition\": ${replicas}}}}} | jq > ${META_PATCH_FILE}
  kubectl patch sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE} --patch-file ${META_PATCH_FILE}
  log "[restart_meta] workload ${workload} restart done!"
}

restart_storage() {
  log "[restart_storage] restart storage service"
  local workload="${NEBULA_CLUSTER_NAME}-storaged"
  local replicas=$(get_replicas ${workload} ${NEBULA_CLUSTER_NAMESPACE})
  log "[restart_storage] workload ${workload} replicas is $replicas"
  kubectl rollout restart sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE}
  local partition=$((replicas-1))
  while [ ${partition} -ge 0 ]; do
    echo {\"spec\": {\"updateStrategy\": {\"rollingUpdate\": {\"partition\": ${partition}}}}} | jq > ${STORAGE_PATCH_FILE}
    kubectl patch sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE} --patch-file ${STORAGE_PATCH_FILE}
    sleep ${GRACEFUL_TERMINATING_PERIOD}
    pod_name=${workload}-${partition}
    health_check ${pod_name} ${NEBULA_CLUSTER_NAMESPACE} storage
    partition=$((partition-1))
  done
  echo {\"spec\": {\"updateStrategy\": {\"rollingUpdate\": {\"partition\": ${replicas}}}}} | jq > ${STORAGE_PATCH_FILE}
  kubectl patch sts ${workload} -n ${NEBULA_CLUSTER_NAMESPACE} --patch-file ${STORAGE_PATCH_FILE}
  log "[restart_storage] workload ${workload} restart done!"
}

get_replicas() {
 replicas=$(kubectl get sts $1 -n $2 -o json | jq '.spec.replicas')
 echo $replicas
}

health_check() {
  declare -i retry=0
  for i in {1..10}; do
    log "[health_check] $2/$1 phase $i time"
    phase=$(kubectl get pod $1 -n $2 -o json | jq '.status.phase' | tr -d "\"")
    log "$1 phase is ${phase}"
    if [ ${phase} != "Running" ]; then
      sleep ${RETRY_INTERVAL}
      retry+=1
    else
      break
    fi
  done
  if [ $retry -eq 10 ]; then
    log "[health_check] restart nebula $3 failed"
    exit 1
  fi
}

clear_resource() {
 if [ -f ${GRAPH_PATCH_FILE} ]; then rm -f ${GRAPH_PATCH_FILE}; fi
 if [ -f ${META_PATCH_FILE} ]; then rm -f ${META_PATCH_FILE}; fi
 if [ -f ${STORAGE_PATCH_FILE} ]; then rm -f ${STORAGE_PATCH_FILE}; fi
}

#########################
# Execution sequence
#########################
cmd_check

restart_nebula

clear_resource

ELAPSED_TIME=$((SECONDS - START_TIME))
PRETTY=$(printf '%dh:%dm:%ds\n' $((ELAPSED_TIME / 3600)) $((ELAPSED_TIME % 3600 / 60)) $((ELAPSED_TIME % 60)))

log "End execution of nebula cluster restarting script extension on ${HOSTNAME} in ${PRETTY}"
exit 0