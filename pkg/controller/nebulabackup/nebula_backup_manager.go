/*
Copyright 2024 Vesoft Inc.

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

package nebulabackup

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/remote"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/maputil"
)

var backupNameRegex = regexp.MustCompile(".*(BACKUP_[0-9]+_[0-9]+_[0-9]+_[0-9]+_[0-9]+_[0-9]+).*$")

const backupContainerName = "ng-backup"

type Manager interface {
	// Sync	implements the logic for syncing NebulaBackup.
	Sync(backup *v1alpha1.NebulaBackup) error

	// Clean implements the logic for cleaning backup data.
	Clean(backup *v1alpha1.NebulaBackup) error
}

var _ Manager = (*backupManager)(nil)

type backupManager struct {
	clientSet  kube.ClientSet
	restClient kubernetes.Interface
	recorder   record.EventRecorder
}

func NewBackupManager(clientSet kube.ClientSet, restClient kubernetes.Interface, recorder record.EventRecorder) Manager {
	return &backupManager{
		clientSet:  clientSet,
		restClient: restClient,
		recorder:   recorder,
	}
}

func (bm *backupManager) Sync(backup *v1alpha1.NebulaBackup) error {
	return bm.syncBackupJob(backup)
}

func (bm *backupManager) Clean(backup *v1alpha1.NebulaBackup) error {
	if !backup.CleanBackupData() {
		return nil
	}

	klog.Infof("start to clean backup [%s/%s]", backup.Namespace, backup.Name)

	finished, err := bm.ensureBackupJobFinished(backup)
	if err != nil {
		return fmt.Errorf("ensure backup [%s/%s] job finished failed: %v", backup.Namespace, backup.Name, err)
	}
	if !finished {
		return utilerrors.ReconcileErrorf("waiting for backup [%s/%s] job to finish", backup.Namespace, backup.Name)
	}

	if backup.Status.BackupName == "" {
		klog.Infof("backup [%s/%s] remote backup is empty", backup.Namespace, backup.Name)
		return bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
			Type:   v1alpha1.BackupClean,
			Status: corev1.ConditionTrue,
		}, nil)
	}

	cleanJobName := getCleanJobName(backup.Name)
	job, err := bm.clientSet.Job().GetJob(backup.Namespace, cleanJobName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			clusterNamespace := getClusterNamespace(backup)
			clusterName := backup.Spec.Config.NamespacedObjectReference.ClusterName
			cluster, err := bm.clientSet.NebulaCluster().GetNebulaCluster(clusterNamespace, clusterName)
			if err != nil {
				return err
			}
			// cluster.MetadComponent().IsReady()
			if !cluster.IsReady() {
				return utilerrors.ReconcileErrorf("cluster [%s/%s] is not ready", clusterNamespace, clusterName)
			}

			if err := bm.runCleanJob(backup, cluster); err != nil {
				return err
			}

			if err := bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
				Type:   v1alpha1.BackupClean,
				Status: corev1.ConditionFalse,
			}, nil); err != nil {
				return err
			}
			return utilerrors.ReconcileErrorf("backup [%s/%s] clean job is running", backup.Namespace, backup.Name)
		}
		return err
	}

	if job != nil && isJobComplete(job) {
		return bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
			Type:   v1alpha1.BackupClean,
			Status: corev1.ConditionTrue,
		}, nil)
	}

	failed, reason, originalReason, err := bm.detectCleanJobFailure(backup)
	if err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("detect failure, reason: %s, original reason: %s", reason, originalReason)
	}

	return utilerrors.ReconcileErrorf("waiting for backup [%s/%s] clean job to finish", backup.Namespace, backup.Name)
}

func (bm *backupManager) syncBackupJob(backup *v1alpha1.NebulaBackup) error {
	backupType := getBackupType(backup.Spec.Config.BaseBackupName)
	jobName := getBackupJobName(backup.Name, backupType.String())
	job, err := bm.clientSet.Job().GetJob(backup.Namespace, jobName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			clusterNamespace := getClusterNamespace(backup)
			clusterName := backup.Spec.Config.NamespacedObjectReference.ClusterName
			cluster, err := bm.clientSet.NebulaCluster().GetNebulaCluster(clusterNamespace, clusterName)
			if err != nil {
				return err
			}
			if !cluster.IsReady() {
				return utilerrors.ReconcileErrorf("cluster [%s/%s] is not ready", clusterNamespace, clusterName)
			}

			if err := bm.runBackupJob(backup, cluster); err != nil {
				return err
			}

			if err := bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
				Type:   v1alpha1.BackupRunning,
				Status: corev1.ConditionTrue,
			}, &kube.BackupUpdateStatus{
				Type:          backupType,
				TimeStarted:   &metav1.Time{Time: time.Now()},
				ConditionType: v1alpha1.BackupRunning,
			}); err != nil {
				return err
			}
			return utilerrors.ReconcileErrorf("backup [%s/%s] job is running", backup.Namespace, backup.Name)
		}
		return err
	}

	if backup.Status.BackupName == "" {
		logs, err := kube.GetPodLog(context.Background(), bm.restClient, backup.Namespace, jobName, backupContainerName)
		if err != nil {
			if err == kube.ErrInitializedNotReady || err == kube.ErrContainerCreating {
				klog.Errorf("failed to get logs: %v", err)
				return utilerrors.ReconcileErrorf("waiting for backup [%s/%s] container running", backup.Namespace, backup.Name)
			}
			return err
		}
		backupName := findMatchedBackupName(logs)
		klog.Infof("backup [%s/%s] find matched backup %s from logs", backup.Namespace, backup.Name, backupName)
		if backupName != "" {
			if err := bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, nil,
				&kube.BackupUpdateStatus{BackupName: pointer.String(backupName)}); err != nil {
				return err
			}
		}
	}

	if job != nil && isJobComplete(job) {
		if err := bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
			Type:   v1alpha1.BackupComplete,
			Status: corev1.ConditionTrue,
		}, &kube.BackupUpdateStatus{
			TimeCompleted: &metav1.Time{Time: time.Now()},
			ConditionType: v1alpha1.BackupComplete,
		}); err != nil {
			return err
		}
		if pointer.BoolDeref(backup.Spec.AutoRemoveFinished, false) {
			return bm.clientSet.Job().DeleteJob(backup.Namespace, jobName)
		}
	}

	failed, reason, originalReason, err := bm.detectBackupJobFailure(backup)
	if err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("detect failure, reason: %s, original reason: %s", reason, originalReason)
	}

	return utilerrors.ReconcileErrorf("waiting for backup [%s/%s] job to finish", backup.Namespace, backup.Name)
}

func (bm *backupManager) ensureBackupJobFinished(backup *v1alpha1.NebulaBackup) (bool, error) {
	backupType := getBackupType(backup.Spec.Config.BaseBackupName)
	backupJobName := getBackupJobName(backup.Name, backupType.String())
	backupJob, err := bm.clientSet.Job().GetJob(backup.Namespace, backupJobName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
	}

	if backupJob.DeletionTimestamp != nil {
		klog.Infof("backup [%s/%s] job %s is deleting, cleaner will wait", backup.Namespace, backup.Name, backupJobName)
		return false, nil
	}

	if isJobFinished(backupJob) {
		return true, nil
	}

	if backup.Status.BackupName == "" {
		klog.Infof("backup [%s/%s] job %s is running, cleaner need sync remote backup", backup.Namespace, backup.Name, backupJobName)
		return false, nil
	}

	klog.Infof("backup [%s/%s] job %s is running, cleaner will delete it and wait it done", backup.Namespace, backup.Name, backupJobName)
	if err := bm.clientSet.Job().DeleteJob(backup.Namespace, backupJobName); err != nil {
		return false, err
	}

	return true, nil
}

func (bm *backupManager) detectBackupJobFailure(backup *v1alpha1.NebulaBackup) (
	failed bool, reason string, originalReason string, err error) {
	backupType := getBackupType(backup.Spec.Config.BaseBackupName)
	backupJobName := getBackupJobName(backup.Name, backupType.String())
	backupLabel := label.New().Backup(backup.Name).BackupJob()
	failed, reason, originalReason, err = bm.isPodOrJobFailure(backup.Namespace, backupJobName, backupLabel)
	if err != nil {
		klog.Errorf("failed to check backup [%s/%s] pod or job status: %v", backup.Namespace, backup.Name, err)
		return false, "", "", err
	}
	if !failed {
		return false, "", "", nil
	}
	return failed, reason, originalReason, nil
}

func (bm *backupManager) detectCleanJobFailure(backup *v1alpha1.NebulaBackup) (
	failed bool, reason string, originalReason string, err error) {
	cleanJobName := getCleanJobName(backup.Name)
	backupLabel := label.New().Backup(backup.Name).CleanJob()
	failed, reason, originalReason, err = bm.isPodOrJobFailure(backup.Namespace, cleanJobName, backupLabel)
	if err != nil {
		klog.Errorf("failed to check backup [%s/%s] pod or job status: %v", backup.Namespace, backup.Name, err)
		return false, "", "", err
	}
	if !failed {
		return false, "", "", nil
	}
	return failed, reason, originalReason, nil
}

func (bm *backupManager) isPodOrJobFailure(namespace, jobName string, label label.Label) (
	failed bool, reason string, originalReason string, err error) {
	selector, err := label.Selector()
	if err != nil {
		return false, "", "", err
	}
	pods, err := bm.clientSet.Pod().ListPods(namespace, selector)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, "", "", err
	}
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodFailed {
			reason = "PodFailed"
			originalReason = getContainerTerminateReason(pod, backupContainerName)
			return true, reason, originalReason, nil
		}
	}

	job, err := bm.clientSet.Job().GetJob(namespace, jobName)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, "", "", err
	}
	if job != nil {
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				reason = "JobFailed"
				originalReason = condition.Reason
				return true, reason, originalReason, nil
			}
		}
	}
	return false, "", "", nil
}

func (bm *backupManager) runBackupJob(backup *v1alpha1.NebulaBackup, cluster *v1alpha1.NebulaCluster) error {
	backupType := getBackupType(backup.Spec.Config.BaseBackupName)
	endpoints := cluster.GetMetadEndpoints(v1alpha1.MetadPortNameThrift)
	cmd := []string{"/bin/sh", "-ecx"}
	flags := fmt.Sprintf("exec /usr/local/bin/br-ent backup %s --meta %s --concurrency %d", backupType.String(), endpoints[0], backup.Spec.Config.Concurrency)
	if backup.Spec.Config.BaseBackupName != nil {
		flags += " --base " + *backup.Spec.Config.BaseBackupName
	}
	storageFlags, err := bm.getStorageFlags(backup.Namespace, backup.Spec.Config.StorageProvider)
	if err != nil {
		return err
	}
	flags += storageFlags
	sslFlags := bm.getSslFlags(cluster)
	if sslFlags != "" {
		flags += sslFlags
	}
	cmd = append(cmd, flags)

	backupLabel := label.New().Backup(backup.Name).BackupJob()
	labels := maputil.MergeStringMaps(true, backupLabel, backup.Labels)
	jobName := getBackupJobName(backup.Name, backupType.String())
	job := buildBackupJob(backup, cluster, jobName, cmd, labels)

	return bm.clientSet.Job().CreateJob(job)
}

func (bm *backupManager) runCleanJob(backup *v1alpha1.NebulaBackup, cluster *v1alpha1.NebulaCluster) error {
	endpoints := cluster.GetMetadEndpoints(v1alpha1.MetadPortNameThrift)
	cmd := []string{"/bin/sh", "-ecx"}
	flags := fmt.Sprintf("exec /usr/local/bin/br-ent cleanup --name %s --meta %s", backup.Status.BackupName, endpoints[0])
	storageFlags, err := bm.getStorageFlags(backup.Namespace, backup.Spec.Config.StorageProvider)
	if err != nil {
		return err
	}
	flags += storageFlags
	sslFlags := bm.getSslFlags(cluster)
	if sslFlags != "" {
		flags += sslFlags
	}
	cmd = append(cmd, flags)

	backupLabel := label.New().Backup(backup.Name).CleanJob()
	labels := maputil.MergeStringMaps(true, backupLabel, backup.Labels)
	jobName := getCleanJobName(backup.Name)
	job := buildBackupJob(backup, cluster, jobName, cmd, labels)

	return bm.clientSet.Job().CreateJob(job)
}

func (bm *backupManager) getSslFlags(cluster *v1alpha1.NebulaCluster) string {
	var flag string
	if cluster.IsMetadSSLEnabled() || cluster.IsClusterSSLEnabled() {
		flag += " --enable-ssl"
	}
	if cluster.InsecureSkipVerify() {
		flag += " --insecure-skip-verify"
	}
	if cluster.SslServerName() != "" {
		flag += " --server-name=" + cluster.Spec.SSLCerts.ServerName
	}
	return flag
}

func (bm *backupManager) getStorageFlags(namespace string, provider v1alpha1.StorageProvider) (string, error) {
	var storageFlags string
	storageType := remote.GetStorageType(provider)
	switch storageType {
	case v1alpha1.ObjectStorageS3:
		accessKey, secretKey, err := remote.GetS3Key(bm.clientSet, namespace, provider.S3.SecretName)
		if err != nil {
			return "", fmt.Errorf("get S3 key failed: %v", err)
		}
		storageFlags = fmt.Sprintf(" --storage s3://%s --s3.region %s --s3.endpoint %s --s3.access_key %s --s3.secret_key %s",
			provider.S3.Bucket, provider.S3.Region, provider.S3.Endpoint, accessKey, secretKey)
	case v1alpha1.ObjectStorageGS:
		credentials, err := remote.GetGsCredentials(bm.clientSet, namespace, provider.GS.SecretName)
		if err != nil {
			return "", fmt.Errorf("get GS credentials failed: %v", err)
		}
		storageFlags = fmt.Sprintf(` --storage gs://%s --gs.credentials '%s'`, provider.GS.Bucket, credentials)
	default:
		return "", fmt.Errorf("unknown storage type: %s", storageType)
	}
	return storageFlags, nil
}

func buildBackupJob(backup *v1alpha1.NebulaBackup, cluster *v1alpha1.NebulaCluster, jobName string,
	cmd []string, labels label.Label) *batchv1.Job {
	mountCertSecret := (cluster.IsMetadSSLEnabled() || cluster.IsClusterSSLEnabled()) && !v1alpha1.EnableLocalCerts()
	containers := make([]corev1.Container, 0, 1)
	container := corev1.Container{
		Name:            backupContainerName,
		Image:           fmt.Sprintf("%s:%s", backup.Spec.Image, backup.Spec.Version),
		Command:         cmd,
		ImagePullPolicy: backup.Spec.ImagePullPolicy,
		Env:             backup.Spec.Env,
		Resources:       backup.Spec.Resources,
		VolumeMounts:    backup.Spec.VolumeMounts,
	}
	if mountCertSecret {
		certMounts := v1alpha1.GetClientCertVolumeMounts()
		container.VolumeMounts = append(container.VolumeMounts, certMounts...)
	}
	containers = append(containers, container)

	containers = mergeSidecarContainers([]corev1.Container{container}, backup.Spec.SidecarContainers)

	volumes := backup.Spec.Volumes
	if mountCertSecret {
		certVolumes := v1alpha1.GetClientCertVolumes(cluster.Spec.SSLCerts)
		volumes = append(volumes, certVolumes...)
	}

	podSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: backup.Annotations,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: v1alpha1.NebulaServiceAccountName,
			InitContainers:     backup.Spec.InitContainers,
			Containers:         containers,
			RestartPolicy:      corev1.RestartPolicyNever,
			Tolerations:        backup.Spec.Tolerations,
			ImagePullSecrets:   backup.Spec.ImagePullSecrets,
			Affinity:           backup.Spec.Affinity,
			Volumes:            volumes,
		},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            jobName,
			Namespace:       backup.Namespace,
			Labels:          labels,
			Annotations:     backup.Annotations,
			OwnerReferences: backup.GenerateOwnerReferences(),
		},
		Spec: batchv1.JobSpec{
			Parallelism:  pointer.Int32(1),
			Completions:  pointer.Int32(1),
			BackoffLimit: pointer.Int32(0),
			Template:     podSpec,
		},
	}
	return job
}

func mergeSidecarContainers(origins, injected []corev1.Container) []corev1.Container {
	containersInPod := make(map[string]int)
	for index, container := range origins {
		containersInPod[container.Name] = index
	}

	var appContainers []corev1.Container
	for _, sidecar := range injected {
		if index, ok := containersInPod[sidecar.Name]; ok {
			origins[index] = sidecar
			continue
		}
		appContainers = append(appContainers, sidecar)
	}

	origins = append(origins, appContainers...)
	return origins
}

func getContainerTerminateReason(pod corev1.Pod, containerName string) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			if cs.State.Terminated != nil {
				return cs.State.Terminated.String()
			}
		}
	}
	return ""
}

func getClusterNamespace(backup *v1alpha1.NebulaBackup) string {
	ns := backup.GetNamespace()
	if backup.Spec.Config.ClusterNamespace != nil {
		ns = *backup.Spec.Config.ClusterNamespace
	}
	return ns
}

func isJobFinished(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isJobComplete(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func getBackupType(baseBackupName *string) v1alpha1.BackupType {
	backupType := v1alpha1.BackupTypeFull
	if baseBackupName != nil {
		backupType = v1alpha1.BackupTypeIncr
	}
	return backupType
}

func getCleanJobName(backupName string) string {
	return fmt.Sprintf("clean-%s", backupName)
}

func getBackupJobName(backupName, backupType string) string {
	return fmt.Sprintf("backup-%s-%s", backupType, backupName)
}

func findMatchedBackupName(logs string) string {
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "+") {
			continue
		}
		subMatches := backupNameRegex.FindStringSubmatch(line)
		if len(subMatches) == 2 {
			return subMatches[1]
		}
	}
	return ""
}
