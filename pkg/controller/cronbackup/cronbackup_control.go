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

package cronbackup

import (
	"context"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/async"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
)

// concurrency is the count of goroutines to delete expired backup
const concurrency = 2

type ControlInterface interface {
	UpdateCronBackup(cronBackup *v1alpha1.NebulaCronBackup) (*time.Duration, error)
}

var _ ControlInterface = (*defaultCronBackupControl)(nil)

type defaultCronBackupControl struct {
	clientSet kube.ClientSet
	cbManager Manager
}

func NewCronBackupControl(clientSet kube.ClientSet, cbManager Manager) ControlInterface {
	return &defaultCronBackupControl{
		clientSet: clientSet,
		cbManager: cbManager,
	}
}

func (c *defaultCronBackupControl) UpdateCronBackup(cronBackup *v1alpha1.NebulaCronBackup) (*time.Duration, error) {
	requeueAfter, err := c.cbManager.Sync(cronBackup)
	if err != nil {
		if err := c.clientSet.NebulaCronBackup().UpdateCronBackupStatus(cronBackup.DeepCopy()); err != nil {
			return nil, err
		}
		return nil, err
	}

	backupsToBeReconciled, err := c.getBackupsToBeReconciled(cronBackup)
	if err != nil {
		return nil, err
	}

	if c.cleanupFinishedBackups(cronBackup, backupsToBeReconciled) {
		cronBackup.Status.BackupCleanTime = &metav1.Time{Time: time.Now()}
		if err := c.clientSet.NebulaCronBackup().UpdateCronBackupStatus(cronBackup.DeepCopy()); err != nil {
			return nil, err
		}
	}

	if pointer.BoolDeref(cronBackup.Spec.Pause, false) && requeueAfter == nil {
		requeueAfter, err = c.calculateExpirationTime(cronBackup, backupsToBeReconciled)
		if err != nil {
			return nil, err
		}
	}

	if requeueAfter != nil {
		klog.V(4).Infof("re-queuing cron backup [%s/%s], requeueAfter %v", cronBackup.Namespace, cronBackup.Name, requeueAfter)
		if err := c.clientSet.NebulaCronBackup().UpdateCronBackupStatus(cronBackup.DeepCopy()); err != nil {
			return nil, err
		}
		return requeueAfter, nil
	}

	return nil, nil
}

func (c *defaultCronBackupControl) cleanupFinishedBackups(cronBackup *v1alpha1.NebulaCronBackup, backups []v1alpha1.NebulaBackup) bool {
	if cronBackup.Spec.MaxReservedTime == nil {
		return false
	}

	reservedTime, err := time.ParseDuration(*cronBackup.Spec.MaxReservedTime)
	if err != nil {
		klog.Errorf("cron backup [%s/%s] invalid time format: %v", cronBackup.Namespace, cronBackup.Name, err)
		return false
	}

	ascBackups := sortBackupsByAscOrder(backups)
	expiredBackups := calculateExpiredBackups(ascBackups, reservedTime)
	if len(expiredBackups) == 0 {
		klog.V(4).Infof("cron backup [%s/%s] no expired backups found", cronBackup.Namespace, cronBackup.Name)
		return false
	}

	group := async.NewGroup(context.TODO(), concurrency)
	for i := range expiredBackups {
		backup := expiredBackups[i]
		worker := func() error {
			if backup.DeletionTimestamp != nil {
				return nil
			}
			if err := c.clientSet.NebulaBackup().DeleteNebulaBackup(backup.Namespace, backup.Name); err != nil {
				return err
			}
			klog.V(4).Infof("cron backup [%s/%s] cleanups expired backup %s ago", backup.Namespace, backup.Name, reservedTime.String())
			return nil
		}

		group.Add(func(stopCh chan interface{}) {
			stopCh <- worker()
		})
	}
	if err := group.Wait(); err != nil {
		klog.Errorf("cron backup [%s/%s] failed to cleanup expired backups: %v", cronBackup.Namespace, cronBackup.Name, err)
		return false
	}

	return true
}

func (c *defaultCronBackupControl) getBackupsToBeReconciled(cronBackup *v1alpha1.NebulaCronBackup) ([]v1alpha1.NebulaBackup, error) {
	backupLabel := label.New().CronJob(cronBackup.Name)
	selector, err := backupLabel.Selector()
	if err != nil {
		return nil, err
	}
	backups, err := c.clientSet.NebulaBackup().ListNebulaBackups(cronBackup.Namespace, selector)
	if err != nil {
		return nil, err
	}
	return backups, nil
}

func (c *defaultCronBackupControl) calculateExpirationTime(cronBackup *v1alpha1.NebulaCronBackup, backups []v1alpha1.NebulaBackup) (*time.Duration, error) {
	if cronBackup.Spec.MaxReservedTime == nil {
		return nil, nil
	}
	reservedTime, err := time.ParseDuration(*cronBackup.Spec.MaxReservedTime)
	if err != nil {
		return nil, err
	}
	ascBackups := sortBackupsByAscOrder(backups)
	if len(ascBackups) == 0 {
		return nil, nil
	}
	oldestTime := ascBackups[0].CreationTimestamp
	expiredTime := time.Now().Add(-1 * reservedTime)
	klog.Infof("oldest backup create time %v, clenaup expired time %v", oldestTime.Time, expiredTime)
	timeElapsed := oldestTime.Time.Sub(expiredTime)
	if timeElapsed < 0 {
		klog.Infof("cleanup the oldest backup [%s/%s] right now", ascBackups[0].Namespace, ascBackups[0].Name)
		return pointer.Duration(time.Millisecond * 100), nil
	}
	klog.Infof("the oldest backup [%s/%s] will be cleanup after %s", ascBackups[0].Namespace, ascBackups[0].Name, timeElapsed.String())
	return &timeElapsed, nil
}

func sortBackupsByAscOrder(backups []v1alpha1.NebulaBackup) []*v1alpha1.NebulaBackup {
	ascBackups := make([]*v1alpha1.NebulaBackup, 0)
	for i := range backups {
		backup := backups[i]
		if !(condition.IsBackupFailed(&backup) || condition.IsBackupComplete(&backup)) {
			continue
		}
		if backup.DeletionTimestamp != nil {
			continue
		}
		ascBackups = append(ascBackups, &backup)
	}

	sort.Sort(byBackupStartTime(ascBackups))
	return ascBackups
}

func calculateExpiredBackups(ascBackups []*v1alpha1.NebulaBackup, reservedTime time.Duration) []*v1alpha1.NebulaBackup {
	expiredTS := time.Now().Add(-1 * reservedTime).Unix()
	i := 0
	for ; i < len(ascBackups); i++ {
		startTS := ascBackups[i].CreationTimestamp.Unix()
		if startTS >= expiredTS {
			break
		}
	}
	return ascBackups[:i]
}

// byBackupStartTime sorts a list of backups by start timestamp, using their names as a tiebreaker.
type byBackupStartTime []*v1alpha1.NebulaBackup

func (o byBackupStartTime) Len() int      { return len(o) }
func (o byBackupStartTime) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o byBackupStartTime) Less(i, j int) bool {
	if o[i].Status.TimeStarted == nil && o[j].Status.TimeStarted != nil {
		return false
	}
	if o[i].Status.TimeStarted != nil && o[j].Status.TimeStarted == nil {
		return true
	}
	if o[i].Status.TimeStarted.Equal(o[j].Status.TimeStarted) {
		return o[i].Name < o[j].Name
	}
	return o[i].Status.TimeStarted.Before(o[j].Status.TimeStarted)
}
