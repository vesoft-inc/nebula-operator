/*
Copyright 2021 Vesoft Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

var DynamicFlags = map[string]string{
	"minloglevel":                                 "0",
	"v":                                           "0",
	"accept_partial_success":                      "false",
	"session_reclaim_interval_secs":               "60",
	"max_allowed_query_size":                      "4194304",
	"system_memory_high_watermark_ratio":          "0.8",
	"ng_black_box_file_lifetime_seconds":          "1800",
	"memory_tracker_limit_ratio":                  "0.8",
	"memory_tracker_untracked_reserved_memory_mb": "50",
	"memory_tracker_detail_log":                   "false",
	"memory_tracker_detail_log_interval_ms":       "60000",
	"memory_purge_enabled":                        "true",
	"memory_purge_interval_seconds":               "10",
	"heartbeat_interval_secs":                     "10",
	"raft_heartbeat_interval_secs":                "30",
	"raft_rpc_timeout_ms":                         "500",
	"wal_ttl":                                     "14400",
	"query_concurrently":                          "true",
	"auto_remove_invalid_space":                   "true",
	"snapshot_part_rate_limit":                    "10485760",
	"snapshot_batch_size":                         "1048576",
	"rebuild_index_part_rate_limit":               "4194304",
}

const (
	// nolint: revive
	GraphdConfigTemplate = `
########## basics ##########
# Whether to run as a daemon process
--daemonize=true
# The file to host the process id
--pid_file=pids/nebula-graphd.pid
# Whether to enable optimizer
--enable_optimizer=true
# The default charset when a space is created
--default_charset=utf8
# The default collate when a space is created
--default_collate=utf8_bin
# Heartbeat interval of communication between meta client and graphd service
--heartbeat_interval_secs=10
# Whether to use the configuration obtained from the configuration file
--local_config=true

########## logging ##########
# The directory to host logging files
--log_dir=logs
# Log level, 0, 1, 2, 3 for INFO, WARNING, ERROR, FATAL respectively
--minloglevel=0
# Verbose log level, 1, 2, 3, 4, the higher of the level, the more verbose of the logging
--v=0
# Maximum seconds to buffer the log messages
--logbufsecs=0
# Whether to redirect stdout and stderr to separate output files
--redirect_stdout=true
# Destination filename of stdout and stderr, which will also reside in log_dir.
--stdout_log_file=graphd-stdout.log
--stderr_log_file=graphd-stderr.log
# Copy log messages at or above this level to stderr in addition to logfiles. The numbers of severity levels INFO, WARNING, ERROR, and FATAL are 0, 1, 2, and 3, respectively.
--stderrthreshold=3
# wether logging files' name contain timestamp.
--timestamp_in_logfile_name=true

########## query ##########
# Whether to treat partial success as an error.
# This flag is only used for Read-only access, and Modify access always treats partial success as an error.
--accept_partial_success=false
# Maximum sentence length, unit byte
--max_allowed_query_size=4194304

########## networking ##########
# Comma separated Meta Server Addresses
--meta_server_addrs=127.0.0.1:9559
# Local IP used to identify the nebula-graphd process.
# Change it to an address other than loopback if the service is distributed or
# will be accessed remotely.
--local_ip=127.0.0.1
# Network device to listen on
--listen_netdev=any
# Port to listen on
--port=9669
# To turn on SO_REUSEPORT or not
--reuse_port=false
# Backlog of the listen socket, adjust this together with net.core.somaxconn
--listen_backlog=1024
# The number of seconds Nebula service waits before closing the idle connections
--client_idle_timeout_secs=28800
# The number of seconds before idle sessions expire
# The range should be in [1, 604800]
--session_idle_timeout_secs=28800
# The number of threads to accept incoming connections
--num_accept_threads=1
# The number of networking IO threads, 0 for # of CPU cores
--num_netio_threads=0
# Max active connections for all networking threads. 0 means no limit.
# Max connections for each networking thread = num_max_connections / num_netio_threads
--num_max_connections=0
# The number of threads to execute user queries, 0 for # of CPU cores
--num_worker_threads=0
# HTTP service ip
--ws_ip=0.0.0.0
# HTTP service port
--ws_http_port=19669
# storage client timeout
--storage_client_timeout_ms=60000
# Enable slow query records
--enable_record_slow_query=true
# The number of slow query records
--slow_query_limit=100
# slow query threshold in us
--slow_query_threshold_us=200000
# Port to listen on Meta with HTTP protocol, it corresponds to ws_http_port in metad's configuration file
--ws_meta_http_port=19559

########## authentication ##########
# Enable authorization
--enable_authorize=false
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

########## memory ##########
# System memory high watermark ratio, cancel the memory checking when the ratio greater than 1.0
--system_memory_high_watermark_ratio=0.8

########## audit ##########
# This variable is used to enable audit. The value can be 'true' or 'false'.
--enable_audit=false
# This variable is used to configure where the audit log will be written. Optional：[ file | es ]
# If it is set to 'file', the log will be written into a file specified by audit_log_file variable.
# If it is set to 'es', the audit log will be written to Elasticsearch.
--audit_log_handler=file
# This variable is used to specify the filename that’s going to store the audit log.
# It can contain the path relative to the install dir or absolute path.
# This variable has effect only when audit_log_handler is set to 'file'.
--audit_log_file=./logs/audit/audit.log
# This variable is used to specify the audit log strategy, Optional：[ asynchronous｜ synchronous ]
# asynchronous: log using memory buffer, do not block the main thread
# synchronous: log directly to file, flush and sync every event
# Caution: For performance reasons, when the buffer is full and has not been flushed to the disk,
# the 'asynchronous' mode will discard subsequent requests.
# This variable has effect only when audit_log_handler is set to 'file'.
--audit_log_strategy=synchronous
# This variable can be used to specify the size of memory buffer used for logging,
# used when audit_log_strategy variable is set to 'asynchronous' values.
# This variable has effect only when audit_log_handler is set to 'file'. Uint: B
--audit_log_max_buffer_size=1048576
# This variable is used to specify the audit log format. Supports three log formats [ xml | json | csv ]
# This variable has effect only when audit_log_handler is set to 'file'.
--audit_log_format=xml
# This variable can be used to specify the comma-seperated list of Elasticsearch addresses,
# eg, '192.168.0.1:7001, 192.168.0.2:7001'.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_address=
# This variable can be used to specify the user name of the Elasticsearch.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_user=
# This variable can be used to specify the user password of the Elasticsearch.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_password=
# This variable can be used to specify the number of logs which are sent to Elasticsearch at one time.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_batch_size=1000
# This variable is used to specify the list of spaces for not tracking.
# The value can be comma separated list of spaces, ie, 'nba, basketball'.
--audit_log_exclude_spaces=
# This variable is used to specify the list of log categories for tracking, eg, 'login, ddl'.
# There are eight categories for tracking. There are: [ login ｜ exit | ddl | dql | dml | dcl | util | unknown ].
--audit_log_categories=login,exit

########## metrics ##########
--enable_space_level_metrics=false

########## experimental feature ##########
# if use experimental features
--enable_experimental_feature=false

########## Black box ########
# Enable black box
--ng_black_box_switch=true
# Black box log folder
--ng_black_box_home=black_box
# Black box dump metrics log period
--ng_black_box_dump_period_seconds=5
# Black box log files expire time
--ng_black_box_file_lifetime_seconds=1800

########## session ##########
# Maximum number of sessions that can be created per IP and per user
--max_sessions_per_ip_per_user=300

########## memory tracker ##########
# trackable memory ratio (trackable_memory / (total_memory - untracked_reserved_memory) )
--memory_tracker_limit_ratio=0.8
# untracked reserved memory in Mib
--memory_tracker_untracked_reserved_memory_mb=50

# enable log memory tracker stats periodically
--memory_tracker_detail_log=false
# log memory tacker stats interval in milliseconds
--memory_tracker_detail_log_interval_ms=60000

# enable memory background purge (if jemalloc is used)
--memory_purge_enabled=true
# memory background purge interval in seconds
--memory_purge_interval_seconds=10

########## performance optimization ##########
# The max job size in multi job mode
--max_job_size=1
# The min batch size for handling dataset in multi job mode, only enabled when max_job_size is greater than 1
--min_batch_size=8192
# if true, return directly without go through RPC
--optimize_appendvertices=false
# number of paths constructed by each thread
--path_batch_size=10000

########## HTTP2 ##########
# Enable HTTP2 handler for RPC
--enable_http2_routing=false
# HTTP stream timeout in milliseconds
--stream_timeout_ms=30000

########## SSL ##########
# whether to enable ssl
--enable_ssl=false
# whether to enable ssl of graph server
--enable_graph_ssl=false
# whether to enable ssl of meta server
--enable_meta_ssl=false
# path to cert pem
--cert_path=certs/server.crt
# path to cert key
--key_path=certs/server.key
# path to trusted CA file
--ca_path=certs/ca.crt
# path to trusted client CA file
--ca_client_path=certs/ca.crt
# path to SSL req challenge password
--password_path=certs/password
# path to SSL config watch path of file or directory
--ssl_watch_path=certs
`
	// nolint: revive
	MetadhConfigTemplate = `
########## basics ##########
# Whether to run as a daemon process
--daemonize=true
# The file to host the process id
--pid_file=pids/nebula-metad.pid

########## logging ##########
# The directory to host logging files
--log_dir=logs
# Log level, 0, 1, 2, 3 for INFO, WARNING, ERROR, FATAL respectively
--minloglevel=0
# Verbose log level, 1, 2, 3, 4, the higher of the level, the more verbose of the logging
--v=0
# Maximum seconds to buffer the log messages
--logbufsecs=0
# Whether to redirect stdout and stderr to separate output files
--redirect_stdout=true
# Destination filename of stdout and stderr, which will also reside in log_dir.
--stdout_log_file=metad-stdout.log
--stderr_log_file=metad-stderr.log
# Copy log messages at or above this level to stderr in addition to logfiles. The numbers of severity levels INFO, WARNING, ERROR, and FATAL are 0, 1, 2, and 3, respectively.
--stderrthreshold=3
# wether logging files' name contain time stamp, If Using logrotate to rotate logging files, than should set it to true.
--timestamp_in_logfile_name=true

########## networking ##########
# Comma separated Meta Server addresses
--meta_server_addrs=127.0.0.1:9559
# Local IP used to identify the nebula-metad process.
# Change it to an address other than loopback if the service is distributed or
# will be accessed remotely.
--local_ip=127.0.0.1
# Meta daemon listening port
--port=9559
# HTTP service ip
--ws_ip=0.0.0.0
# HTTP service port
--ws_http_port=19559
# Port to listen on Storage with HTTP protocol, it corresponds to ws_http_port in storage's configuration file
--ws_storage_http_port=19779

########## storage ##########
# Root data path, here should be only single path for metad
--data_path=data/meta

# !!! Minimum reserved bytes of data path
--minimum_reserved_bytes=268435456

########## Misc #########
# The default number of parts when a space is created
--default_parts_num=10
# The default replica factor when a space is created
--default_replica_factor=1

--heartbeat_interval_secs=10
--agent_heartbeat_interval_secs=60

############## rocksdb Options ##############
--rocksdb_wal_sync=true

########## Black box ########
# Enable black box
--ng_black_box_switch=true
# Black box log folder
--ng_black_box_home=black_box
# Black box dump metrics log period
--ng_black_box_dump_period_seconds=5
# Black box log files expire time
--ng_black_box_file_lifetime_seconds=1800

########## SSL ##########
# whether to enable ssl
--enable_ssl=false
# whether to enable ssl of meta server
--enable_meta_ssl=false
# path to cert pem
--cert_path=certs/server.crt
# path to cert key
--key_path=certs/server.key
# path to trusted CA file
--ca_path=certs/ca.crt
# path to trusted client CA file
--ca_client_path=certs/ca.crt
# path to SSL req challenge password
--password_path=certs/password
# path to SSL config watch path of file or directory
--ssl_watch_path=certs
`
	// nolint: revive
	StoragedConfigTemplate = `
########## basics ##########
# Whether to run as a daemon process
--daemonize=true
# The file to host the process id
--pid_file=pids/nebula-storaged.pid
# Whether to use the configuration obtained from the configuration file
--local_config=true

########## logging ##########
# The directory to host logging files
--log_dir=logs
# Log level, 0, 1, 2, 3 for INFO, WARNING, ERROR, FATAL respectively
--minloglevel=0
# Verbose log level, 1, 2, 3, 4, the higher of the level, the more verbose of the logging
--v=0
# Maximum seconds to buffer the log messages
--logbufsecs=0
# Whether to redirect stdout and stderr to separate output files
--redirect_stdout=true
# Destination filename of stdout and stderr, which will also reside in log_dir.
--stdout_log_file=storaged-stdout.log
--stderr_log_file=storaged-stderr.log
# Copy log messages at or above this level to stderr in addition to logfiles. The numbers of severity levels INFO, WARNING, ERROR, and FATAL are 0, 1, 2, and 3, respectively.
--stderrthreshold=3
# Wether logging files' name contain time stamp.
--timestamp_in_logfile_name=true

########## networking ##########
# Comma separated Meta server addresses
--meta_server_addrs=127.0.0.1:9559
# Local IP used to identify the nebula-storaged process.
# Change it to an address other than loopback if the service is distributed or
# will be accessed remotely.
--local_ip=127.0.0.1
# Storage daemon listening port
--port=9779
# HTTP service ip
--ws_ip=0.0.0.0
# HTTP service port
--ws_http_port=19779
# heartbeat with meta service
--heartbeat_interval_secs=10

######### Raft #########
# Raft election timeout
--raft_heartbeat_interval_secs=30
# RPC timeout for raft client (ms)
--raft_rpc_timeout_ms=500
# recycle Raft WAL
--wal_ttl=14400
# whether send raft snapshot by files via http
--snapshot_send_files=true

########## Disk ##########
# Root data path. Split by comma. e.g. --data_path=/disk1/path1/,/disk2/path2/
# One path per Rocksdb instance.
--data_path=data/storage

# Minimum reserved bytes of each data path
--minimum_reserved_bytes=268435456

# The default reserved bytes for one batch operation
--rocksdb_batch_size=4096
# The default block cache size used in BlockBasedTable.
# The unit is MB.
--rocksdb_block_cache=4096
# Disable page cache to better control memory used by rocksdb.
# Caution: Make sure to allocate enough block cache if disabling page cache!
--disable_page_cache=false
# The type of storage engine, rocksdb, memory, etc.
--engine_type=rocksdb

# Compression algorithm, options: no,snappy,lz4,lz4hc,zlib,bzip2,zstd
# For the sake of binary compatibility, the default value is snappy.
# Recommend to use:
#   * lz4 to gain more CPU performance, with the same compression ratio with snappy
#   * zstd to occupy less disk space
#   * lz4hc for the read-heavy write-light scenario
--rocksdb_compression=lz4

# Set different compressions for different levels
# For example, if --rocksdb_compression is snappy,
# "no:no:lz4:lz4::zstd" is identical to "no:no:lz4:lz4:snappy:zstd:snappy"
# In order to disable compression for level 0/1, set it to "no:no"
--rocksdb_compression_per_level=

# Whether or not to enable rocksdb's statistics, disabled by default
--enable_rocksdb_statistics=false

# Statslevel used by rocksdb to collection statistics, optional values are
#   * kExceptHistogramOrTimers, disable timer stats, and skip histogram stats
#   * kExceptTimers, Skip timer stats
#   * kExceptDetailedTimers, Collect all stats except time inside mutex lock AND time spent on compression.
#   * kExceptTimeForMutex, Collect all stats except the counters requiring to get time inside the mutex lock.
#   * kAll, Collect all stats
--rocksdb_stats_level=kExceptHistogramOrTimers

# Whether or not to enable rocksdb's prefix bloom filter, enabled by default.
--enable_rocksdb_prefix_filtering=true
# Whether or not to enable rocksdb's whole key bloom filter, disabled by default.
--enable_rocksdb_whole_key_filtering=false

############## rocksdb Options ##############
# rocksdb DBOptions in json, each name and value of option is a string, given as "option_name":"option_value" separated by comma
--rocksdb_db_options={}
# rocksdb ColumnFamilyOptions in json, each name and value of option is string, given as "option_name":"option_value" separated by comma
--rocksdb_column_family_options={"write_buffer_size":"67108864","max_write_buffer_number":"4","max_bytes_for_level_base":"268435456"}
# rocksdb BlockBasedTableOptions in json, each name and value of option is string, given as "option_name":"option_value" separated by comma
--rocksdb_block_based_table_options={"block_size":"8192"}

############## storage cache ##############
# Whether to enable storage cache
--enable_storage_cache=false
# Total capacity reserved for storage in memory cache in MB
--storage_cache_capacity=0
# Estimated number of cache entries on this storage node in base 2 logarithm. E.g., in case of 20, the estimated number of entries will be 2^20. 
# A good estimate can be log2(#vertices on this storage node). The maximum allowed is 31.
--storage_cache_entries_power=20

# Whether to add vertex pool in cache. Only valid when storage cache is enabled.
--enable_vertex_pool=false
# Vertex pool size in MB
--vertex_pool_capacity=50
# TTL in seconds for vertex items in the cache
--vertex_item_ttl=300

# Whether to add negative pool in cache. Only valid when storage cache is enabled.
--enable_negative_pool=false
# Negative pool size in MB
--negative_pool_capacity=50
# TTL in seconds for negative items in the cache
--negative_item_ttl=300

############### misc ####################
# Whether turn on query in multiple thread
--query_concurrently=true
# Whether remove outdated space data
--auto_remove_invalid_space=true
# Network IO threads number
--num_io_threads=16
# Max active connections for all networking threads. 0 means no limit.
# Max connections for each networking thread = num_max_connections / num_netio_threads
--num_max_connections=0
# Worker threads number to handle request
--num_worker_threads=32
# Maximum subtasks to run admin jobs concurrently
--max_concurrent_subtasks=10
# The rate limit in bytes when leader synchronizes snapshot data
--snapshot_part_rate_limit=10485760
# The amount of data sent in each batch when leader synchronizes snapshot data
--snapshot_batch_size=1048576
# The rate limit in bytes when leader synchronizes rebuilding index
--rebuild_index_part_rate_limit=4194304
# The amount of data sent in each batch when leader synchronizes rebuilding index
--rebuild_index_batch_size=1048576

############## non-volatile cache ##############
# Cache file location
--nv_cache_path=/tmp/cache
# Cache file size in MB
--nv_cache_size=0
# DRAM part size of non-volatile cache in MB
--nv_dram_size=50
# DRAM part bucket power. The value is a logarithm with a base of 2. Optional values are 0-32.
--nv_bucket_power=20
# DRAM part lock power. The value is a logarithm with a base of 2. The recommended value is max(1, nv_bucket_power - 10).
--nv_lock_power=10

########## Black box ########
# Enable black box
--ng_black_box_switch=true
# Black box log folder
--ng_black_box_home=black_box
# Black box dump metrics log period
--ng_black_box_dump_period_seconds=5
# Black box log files expire time
--ng_black_box_file_lifetime_seconds=1800

########## memory tracker ##########
# trackable memory ratio (trackable_memory / (total_memory - untracked_reserved_memory) )
--memory_tracker_limit_ratio=0.8
# untracked reserved memory in Mib
--memory_tracker_untracked_reserved_memory_mb=50

# enable log memory tracker stats periodically
--memory_tracker_detail_log=false
# log memory tacker stats interval in milliseconds
--memory_tracker_detail_log_interval_ms=60000

# enable memory background purge (if jemalloc is used)
--memory_purge_enabled=true
# memory background purge interval in seconds
--memory_purge_interval_seconds=10

########## SSL ##########
# whether to enable ssl
--enable_ssl=false
# whether to enable ssl of meta server
--enable_meta_ssl=false
# path to cert pem
--cert_path=certs/server.crt
# path to cert key
--key_path=certs/server.key
# path to trusted CA file
--ca_path=certs/ca.crt
# path to trusted client CA file
--ca_client_path=certs/ca.crt
# path to SSL req challenge password
--password_path=certs/password
# path to SSL config watch path of file or directory
--ssl_watch_path=certs
`
)
