## Backup & Restore

### Requirements

* The feature is used for enterprise version
* The NebulaGraph cluster is running
* Operator version >= 1.4.0
* Set the field `enableBR` ot __true__
* Sufficient computational resources can be scheduled to restore NebulaGraph cluster(only restore scenario needed)
* GCS、S3 credentials 

## Backup cluster

#### Features:

* Support full, incremental backup
* Support GCS、S3 protocol compatible storage (AWS S3，Minio, etc.)
* Support the cleanup policy of expired backup
* Support cron backup and can be paused

#### Backup

The fields in the table is optional.

| Parameter            | Description                                                               | Default  |
|:---------------------|:--------------------------------------------------------------------------|:---------|
| `image`              | backup container image without tag, and use `version` as tag              | ``       |
| `nebula.version`     | backup image tag                                                          | ``       |
| `imagePullPolicy`    | backup image pull policy                                                  | `Always` |
| `imagePullSecrets`   | The secret to use for pulling the images                                  | `[]`     |
| `env`                | backup container environment variables                                    | `[]`     |
| `resources`          | backup pod resources                                                      | `{}`     |
| `nodeSelector`       | backup pod nodeSelector                                                   | `{}`     |
| `tolerations`        | backup pod tolerations                                                    | `[]`     |
| `affinity`           | backup pod affinity                                                       | `{}`     |
| `initContainers`     | backup pod init containers                                                | `[]`     |
| `sidecarContainers`  | backup pod sidecar containers                                             | `[]`     |
| `volumes`            | backup pod volumes                                                        | `[]`     |
| `volumeMounts`       | backup pod volume mounts                                                  | `[]`     |
| `cleanBackupData`    | Whether to clean backup data when the object is deleted from the cluster  | `false`  |
| `autoRemoveFinished` | The job that status is failed and completed will be removed automatically | `false`  |
| `config`             | backup cluster config                                                     | `{}`     |

Here is the [nebulabackup-gs.yaml](../../config/samples/nebulabackup-gs.yaml) example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gcp-secret
type: Opaque
data:
  credentials: <GOOGLE_APPLICATION_CREDENTIALS_JSON>
---
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaBackup
metadata:
  name: nb1024
spec:
  image: reg.vesoft-inc.com/cloud-dev/br-ent
  version: v3.7.0
  resources:
    limits:
      cpu: "200m"
      memory: 300Mi
    requests:
      cpu: 100m
      memory: 200Mi
  imagePullSecrets:
  - name: nebula-image
  # The job that status is failed and completed will be removed automatically.
  autoRemoveFinished: true
  # CleanBackupData denotes whether to clean backup data when the object is deleted from the cluster,
  # if not set, the backup data will be retained
  cleanBackupData: true
  config:
    # The name of the backup/restore nebula cluster
    clusterName: nebula
    gs:
      # Location in which the gs bucket is located.
      location: "us-central1"
      #  Bucket in which to store the backup data.
      bucket: "nebula-test"
      # SecretName is the name of secret which stores google application credentials.
      # Secret key: credentials
      secretName: "gcp-secret"
```

```shell
$ kubectl apply -f nebulabackup-gs.yaml
$ kubectl get nb nb1024
NAME     TYPE   BACKUP                       STATUS     STARTED   COMPLETED   AGE
nb1024   full   BACKUP_2024_02_26_08_05_13   Complete   71s       1s          71s
```

#### Cron backup

Here is the [cronbackup.yaml](../../config/samples/cronbackup.yaml) example:

```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCronBackup
metadata:
  name: cron123
spec:
  # The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron
  schedule: "*/5 * * * *"
  # MaxReservedTime is to specify how long backups we want to keep.
  # It should be a duration string format.
  maxReservedTime: 60m
  # Specifies the backup that will be created when executing a CronBackup.
  backupTemplate:
    image: reg.vesoft-inc.com/cloud-dev/br-ent
    version: v3.7.0
    resources:
      limits:
        cpu: "200m"
        memory: 300Mi
      requests:
        cpu: 100m
        memory: 200Mi
    imagePullSecrets:
    - name: nebula-image
    autoRemoveFinished: true
    cleanBackupData: true
    config:
      clusterName: nebula
      gs:
        location: "us-central1"
        bucket: "nebula-test"
        secretName: "gcp-secret"
```

```shell
$ kubectl apply -f cronjob.yaml
$ kubectl get ncb
NAME      SCHEDULE      LASTBACKUP                LASTSCHEDULETIME   LASTSUCCESSFULTIME   BACKUPCLEANTIME   AGE
cron123   */5 * * * *   cron123-20240228t102500   2m40s              64s                  54s               45m
 
$ kubectl get nb -l "apps.nebula-graph.io/cron-backup=cron123"
NAME                      TYPE   BACKUP                       STATUS     STARTED   COMPLETED   AGE
cron123-20240228t094500   full   BACKUP_2024_02_28_09_45_01   Complete   42m       41m         42m
cron123-20240228t102500   full   BACKUP_2024_02_28_10_26_08   Complete   85s       55s         85s
```

## Restore cluster

The fields in the table is optional.

| Parameter            | Description                                                        | Default  |
|:---------------------|:-------------------------------------------------------------------|:---------|
| `nodeSelector`       | restored nebula cluster nodeSelector                               | `{}`     |
| `tolerations`        | restored nebula cluster tolerations                                | `[]`     |
| `affinity`           | restored nebula cluster affinity                                   | `{}`     |
| `autoRemoveFailed`   | The nebula cluster will be removed automatically if restore failed | `false`  |
| `config`             | backup cluster config                                              | `{}`     |


Here is the [nebularestore-s3.yaml](../../config/samples/nebularestore-s3.yaml) example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-secret
type: Opaque
data:
  access_key: <ACCESS_KEY>
  secret_key: <SECRET_KEY>
---
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaRestore
metadata:
  name: nr2048
spec:
  config:
    # The name of the backup/restore nebula cluster
    clusterName: nebula
    # The name of the backup file.
    backupName: "BACKUP_2023_02_28_04_36_41"
    # Used to control the number of concurrent file downloads during data restoration. 
    # The default value is 5.
    concurrency: 50
    s3:
      # Region in which the S3 compatible bucket is located.
      region: "us-east-1"
      #  Bucket in which to store the backup data.
      bucket: "nebula-br-test"
      # Endpoint of S3 compatible storage service
      endpoint: "https://s3.us-east-1.amazonaws.com"
      #  SecretName is the name of secret which stores access key and secret key.
      secretName: "aws-secret"
```

```shell
$ kubectl apply -f nebularestore-s3.yaml
$ kubectl  get nr nr2048
NAME    STATUS     RESTORED-CLUSTER   STARTED   COMPLETED   AGE
nr2048  Complete   ng6tdt             14m       6m55s       14m
 
$ kubectl get nc
NAME     READY   GRAPHD-DESIRED   GRAPHD-READY   METAD-DESIRED   METAD-READY   STORAGED-DESIRED   STORAGED-READY   AGE
nebula   True    1                1              3               3             3                  3                45m
ng6tdt   True    1                1              3               3             3                  3                10m
```

**Note:**

The operator won't remove old nebula cluster after restore successfully, you can do it manually.
