/*
Copyright 2024 Vesoft Inc.
Copyright 2024 The Kubernetes Authors.

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
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/maputil"
)

const (
	// backupNameTimeFormat is the time format for generate backup name
	backupNameTimeFormat = "2006-01-02t15-04-05"

	nextScheduleDelta = 100 * time.Millisecond
)

type Manager interface {
	// Sync	implements the logic for syncing NebulaCronBackup.
	Sync(cronBackup *v1alpha1.NebulaCronBackup) (*time.Duration, error)
}

var _ Manager = (*cronBackupManager)(nil)

type cronBackupManager struct {
	clientSet kube.ClientSet
	recorder  record.EventRecorder
	now       func() time.Time
}

func NewCronBackupManager(clientSet kube.ClientSet, recorder record.EventRecorder) Manager {
	return &cronBackupManager{
		clientSet: clientSet,
		recorder:  recorder,
		now:       time.Now,
	}
}

func (cbm *cronBackupManager) Sync(cb *v1alpha1.NebulaCronBackup) (*time.Duration, error) {
	if cb.DeletionTimestamp != nil {
		return nil, nil
	}

	if pointer.BoolDeref(cb.Spec.Pause, false) {
		klog.V(4).Infof("Not starting backup because the cron backup [%s/%s] is paused", cb.Namespace, cb.Name)
		return nil, nil
	}

	if err := cbm.canPerformNextBackup(cb); err != nil {
		return nil, err
	}

	now := cbm.now()
	sched, err := cron.ParseStandard(cb.Spec.Schedule)
	if err != nil {
		// this is likely a user error in defining the spec value
		// we should log the error and not reconcile this cron backup until an update to spec
		klog.V(2).Infof("cron backup [%s/%s] unparseable schedule: %v", cb.Namespace, cb.Name, err)
		return nil, nil
	}
	scheduledTime, err := nextScheduleTime(cb, now, sched)
	if err != nil {
		// this is likely a user error in defining the spec value
		// we should log the error and not reconcile this cron backup until an update to spec
		klog.V(2).Infof("cron backup [%s/%s] invalid schedule: %v", cb.Namespace, cb.Name, err)
		return nil, nil
	}
	if scheduledTime == nil {
		// The only time this should happen is if queue is filled after restart.
		// Otherwise, the queue is always supposed to trigger sync function at the time of
		// the scheduled time, that will give at least 1 unmet time schedule
		klog.V(4).Infof("cron backup [%s/%s] no unmet start times", cb.Namespace, cb.Name)
		t := nextScheduleTimeDuration(cb, now, sched)
		return t, nil
	}

	backupName, err := cbm.createBackup(cb, *scheduledTime)
	if err != nil {
		return nil, err
	}

	cb.Status.LastBackup = backupName
	cb.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}

	t := nextScheduleTimeDuration(cb, now, sched)
	return t, nil
}

func (cbm *cronBackupManager) canPerformNextBackup(cb *v1alpha1.NebulaCronBackup) error {
	if cb.Status.LastBackup == "" {
		return nil
	}
	backup, err := cbm.clientSet.NebulaBackup().GetNebulaBackup(cb.Namespace, cb.Status.LastBackup)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if condition.IsBackupComplete(backup) || condition.IsBackupFailed(backup) {
		cb.Status.LastSuccessfulTime = backup.Status.TimeCompleted
		return nil
	}
	return utilerrors.ReconcileErrorf("the last backup %s is still running", backup.Name)
}

func (cbm *cronBackupManager) createBackup(cb *v1alpha1.NebulaCronBackup, timestamp time.Time) (string, error) {
	backup := buildBackup(cb, timestamp)
	if err := cbm.clientSet.NebulaBackup().CreateNebulaBackup(backup); err != nil {
		return "", err
	}
	return backup.Name, nil
}

// nextScheduleTimeDuration returns the time duration to requeue based on
// the schedule and last schedule time. It adds a 100ms padding to the next requeue to account
// for Network Time Protocol(NTP) time skews. If the time drifts the adjustment, which in most
// realistic cases should be around 100s, the job will still be executed without missing
// the schedule.
func nextScheduleTimeDuration(cb *v1alpha1.NebulaCronBackup, now time.Time, schedule cron.Schedule) *time.Duration {
	earliestTime, mostRecentTime, _, err := mostRecentScheduleTime(cb, now, schedule)
	if err != nil {
		// we still have to requeue at some point, so aim for the next scheduling slot from now
		mostRecentTime = &now
	} else if mostRecentTime == nil {
		// no missed schedules since earliestTime
		mostRecentTime = &earliestTime
	}

	t := schedule.Next(*mostRecentTime).Add(nextScheduleDelta).Sub(now)
	return &t
}

// nextScheduleTime returns the time.Time of the next schedule after the last scheduled
// and before now, or nil if no unmet schedule times, and an error.
// If there are too many (>100) unstarted times, it will also record a warning.
func nextScheduleTime(cb *v1alpha1.NebulaCronBackup, now time.Time, schedule cron.Schedule) (*time.Time, error) {
	_, mostRecentTime, tooManyMissed, err := mostRecentScheduleTime(cb, now, schedule)
	if mostRecentTime == nil || mostRecentTime.After(now) {
		return nil, err
	}

	if tooManyMissed {
		klog.Infof("cron backup [%s/%s] too many missed times", cb.Namespace, cb.Name)
	}
	return mostRecentTime, err
}

// mostRecentScheduleTime returns:
//   - the last schedule time or CronBackup's creation time,
//   - the most recent time a Backup should be created or nil, if that's after now,
//   - boolean indicating an excessive number of missed schedules,
//   - error in an edge case where the schedule specification is grammatically correct,
//     but logically doesn't make sense (31st day for months with only 30 days, for example).
func mostRecentScheduleTime(cb *v1alpha1.NebulaCronBackup, now time.Time, schedule cron.Schedule) (time.Time, *time.Time, bool, error) {
	earliestTime := cb.ObjectMeta.CreationTimestamp.Time
	if cb.Status.LastScheduleTime != nil {
		earliestTime = cb.Status.LastScheduleTime.Time
	}

	t1 := schedule.Next(earliestTime)
	t2 := schedule.Next(t1)

	if now.Before(t1) {
		return earliestTime, nil, false, nil
	}
	if now.Before(t2) {
		return earliestTime, &t1, false, nil
	}

	// It is possible for cron.ParseStandard("59 23 31 2 *") to return an invalid schedule
	// minute - 59, hour - 23, dom - 31, month - 2, and dow is optional, clearly 31 is invalid
	// In this case the timeBetweenTwoSchedules will be 0, and we error out the invalid schedule
	timeBetweenTwoSchedules := int64(t2.Sub(t1).Round(time.Second).Seconds())
	if timeBetweenTwoSchedules < 1 {
		return earliestTime, nil, false, fmt.Errorf("time difference between two schedules is less than 1 second")
	}
	// this logic used for calculating number of missed schedules does a rough
	// approximation, by calculating a diff between two schedules (t1 and t2),
	// and counting how many of these will fit in between last schedule and now
	timeElapsed := int64(now.Sub(t1).Seconds())
	numberOfMissedSchedules := (timeElapsed / timeBetweenTwoSchedules) + 1

	var mostRecentTime time.Time
	// to get the most recent time accurate for regular schedules and the ones
	// specified with @every form, we first need to calculate the potential earliest
	// time by multiplying the initial number of missed schedules by its interval,
	// this is critical to ensure @every starts at the correct time, this explains
	// the numberOfMissedSchedules-1, the additional -1 serves there to go back
	// in time one more time unit, and let the cron library calculate a proper
	// schedule, for case where the schedule is not consistent, for example
	// something like  30 6-16/4 * * 1-5
	potentialEarliest := t1.Add(time.Duration((numberOfMissedSchedules-1-1)*timeBetweenTwoSchedules) * time.Second)
	for t := schedule.Next(potentialEarliest); !t.After(now); t = schedule.Next(t) {
		mostRecentTime = t
	}

	// An object might miss several starts. For example, if
	// controller gets wedged on friday at 5:01pm when everyone has
	// gone home, and someone comes in on tuesday AM and discovers
	// the problem and restarts the controller, then all the hourly
	// jobs, more than 80 of them for one hourly cronJob, should
	// all start running with no further intervention (if the cronJob
	// allows concurrency and late starts).
	//
	// However, if there is a bug somewhere, or incorrect clock
	// on controller's server or apiservers (for setting creationTimestamp)
	// then there could be so many missed start times (it could be off
	// by decades or more), that it would eat up all the CPU and memory
	// of this controller. In that case, we want to not try to list
	// all the missed start times.
	//
	// I've somewhat arbitrarily picked 100, as more than 80,
	// but less than "lots".
	tooManyMissed := numberOfMissedSchedules > 100

	if mostRecentTime.IsZero() {
		return earliestTime, nil, tooManyMissed, nil
	}
	return earliestTime, &mostRecentTime, tooManyMissed, nil
}

func buildBackup(cb *v1alpha1.NebulaCronBackup, timestamp time.Time) *v1alpha1.NebulaBackup {
	labels := maputil.MergeStringMaps(true, label.New().CronJob(cb.Name), cb.Labels)
	backupSpec := *cb.Spec.BackupTemplate.DeepCopy()
	backup := &v1alpha1.NebulaBackup{
		Spec: backupSpec,
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       cb.Namespace,
			Name:            getBackupName(cb.Name, timestamp),
			Labels:          labels,
			Annotations:     cb.Annotations,
			OwnerReferences: cb.GenerateOwnerReferences(),
		},
	}
	return backup
}

func getBackupName(cronBackupName string, timestamp time.Time) string {
	ts := timestamp.UTC().Format(backupNameTimeFormat)
	t := strings.Replace(ts, "-", "", -1)
	return fmt.Sprintf("%s-%s", cronBackupName, t)
}
