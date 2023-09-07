# Configure custom flags

### Apply custom flags

For each component has a configuration entry, it defines in CRD as config which is a map structure, it will be loaded by configmap.
```go
// Config defines a graphd configuration load into ConfigMap
Config map[string]string `json:"config,omitempty"`
```

The following example will show you how to make configuration changes in CRD, i.e. for any given options `--foo=bar` in conf files, `.config.foo` could be applied like:

```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
  namespace: default
spec:
  graphd:
    resources:
      requests:
        cpu: "500m"
        memory: "500Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 1
    image: vesoft/nebula-graphd
    version: v3.5.0
    storageClaim:
      resources:
        requests:
          storage: 2Gi
      storageClassName: ebs-sc
    config:
      "enable_authorize": "true"
      "auth_type": "password"
      "foo": "bar"
...
```

Afterward, the custom flags  _enable_authorize_, _auth_type_ and _foo_ will be configured and overwritten by configmap.

### Dynamic runtime flags

This a dynamic runtime flags table, the pod rolling update will not be triggered after you apply updates in the scenario:
- All flags updated are in this table

| Flag                                          | Description                                                                                                                               | Default                                                                                                 |
|:----------------------------------------------|:------------------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------------------------------------------------------------------|
| `minloglevel`                                 | Log level, 0, 1, 2, 3 for INFO, WARNING, ERROR, FATAL respectively                                                                        | `0`                                                                                                     |
| `v`                                           | Verbose log level, 1, 2, 3, 4, the higher of the level, the more verbose of the logging                                                   | `0`                                                                                                     |
| `accept_partial_success`                      | This flag is only used for Read-only access, and Modify access always treats partial success as an error                                  | `false`                                                                                                 |
| `session_reclaim_interval_secs`               | Period we try to reclaim expired sessions                                                                                                 | `60`                                                                                                    |
| `max_allowed_query_size`                      | Maximum sentence length, unit byte                                                                                                        | `4194304`                                                                                               |
| `system_memory_high_watermark_ratio`          | System memory high watermark ratio, cancel the memory checking when the ratio greater than 1.0                                            | `0.8`                                                                                                   |
| `ng_black_box_file_lifetime_seconds`          | Black box log files expire time                                                                                                           | `1800`                                                                                                  |
| `memory_tracker_limit_ratio`                  | Trackable memory ratio (trackable_memory / (total_memory - untracked_reserved_memory) )                                                   | `0.8`                                                                                                   |
| `memory_tracker_untracked_reserved_memory_mb` | Untracked reserved memory in Mib                                                                                                          | `50`                                                                                                    |
| `memory_tracker_detail_log`                   | Enable log memory tracker stats periodically                                                                                              | `false`                                                                                                 |
| `memory_tracker_detail_log_interval_ms`       | Log memory tacker stats interval in milliseconds                                                                                          | `60000`                                                                                                 |
| `memory_purge_enabled`                        | Enable memory background purge (if jemalloc is used)                                                                                      | `true`                                                                                                  |
| `memory_purge_interval_seconds`               | Memory background purge interval in seconds                                                                                               | `10`                                                                                                    |
| `heartbeat_interval_secs`                     | Heartbeat interval in seconds                                                                                                             | `10`                                                                                                    |
| `raft_heartbeat_interval_secs`                | Raft election timeout                                                                                                                     | `30`                                                                                                    |
| `raft_rpc_timeout_ms`                         | RPC timeout for raft client (ms)                                                                                                          | `500`                                                                                                   |
| `query_concurrently`                          | Whether turn on query in multiple thread                                                                                                  | `true`                                                                                                  |
| `wal_ttl`                                     | Recycle Raft WAL                                                                                                                          | `14400`                                                                                                 |
| `auto_remove_invalid_space`                   | Whether remove outdated space data                                                                                                        | `true`                                                                                                  |
| `num_io_threads`                              | Network IO threads number                                                                                                                 | `16`                                                                                                    |
| `num_worker_threads`                          | Worker threads number to handle request                                                                                                   | `32`                                                                                                    |
| `max_concurrent_subtasks`                     | Maximum subtasks to run admin jobs concurrently                                                                                           | `10`                                                                                                    |
| `snapshot_part_rate_limit`                    | The rate limit in bytes when leader synchronizes snapshot data                                                                            | `10485760`                                                                                              |
| `snapshot_batch_size`                         | The amount of data sent in each batch when leader synchronizes snapshot data                                                              | `1048576`                                                                                               |
| `rebuild_index_part_rate_limit`               | The rate limit in bytes when leader synchronizes rebuilding index                                                                         | `4194304`                                                                                               |
| `rocksdb_db_options`                          | Rocksdb DBOptions in json, each name and value of option is a string, given as "option_name":"option_value" separated by comma            | `{}`                                                                                                    |
| `rocksdb_column_family_options`               | Rocksdb ColumnFamilyOptions in json, each name and value of option is string, given as "option_name":"option_value" separated by comma    | `{"write_buffer_size":"67108864","max_write_buffer_number":"4","max_bytes_for_level_base":"268435456"}` |
| `rocksdb_block_based_table_options`           | Rocksdb BlockBasedTableOptions in json, each name and value of option is string, given as "option_name":"option_value" separated by comma | `{"block_size":"8192"}`                                                                                 |