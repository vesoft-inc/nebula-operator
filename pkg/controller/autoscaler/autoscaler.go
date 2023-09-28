/*
Copyright 2023 Vesoft Inc.
Copyright 2015 The Kubernetes Authors.

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

package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery/cached/memory"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/restmapper"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller/podautoscaler"
	metricsclient "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	"k8s.io/kubernetes/pkg/controller/util/selectors"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	customclient "k8s.io/metrics/pkg/client/custom_metrics"
	externalclient "k8s.io/metrics/pkg/client/external_metrics"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/autoscaling/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

const (
	defaultTimeout   = 5 * time.Second
	reconcileTimeOut = 10 * time.Second

	controllerName = "nebula-autoscaler"
)

var (
	errNebulaClusterNotFound = errors.New("nebulacluster not found")

	scaleUpLimitFactor  = 2.0
	scaleUpLimitMinimum = 4.0
)

type timestampedRecommendation struct {
	recommendation int32
	timestamp      time.Time
}

type timestampedScaleEvent struct {
	replicaChange int32 // absolute value, non-negative
	timestamp     time.Time
	outdated      bool
}

type HorizontalController struct {
	client                       client.Client
	clientSet                    kube.ClientSet
	replicaCalc                  *podautoscaler.ReplicaCalculator
	eventRecorder                record.EventRecorder
	downscaleStabilisationWindow time.Duration
	podLister                    corelisters.PodLister
	// Latest unstabilized recommendations for each autoscaler.
	recommendations     map[string][]timestampedRecommendation
	recommendationsLock sync.Mutex
	// Latest autoscaler events
	scaleUpEvents       map[string][]timestampedScaleEvent
	scaleUpEventsLock   sync.RWMutex
	scaleDownEvents     map[string][]timestampedScaleEvent
	scaleDownEventsLock sync.RWMutex
	// Storage of HPAs and their selectors.
	hpaSelectors    *selectors.BiMultimap
	hpaSelectorsMux sync.Mutex
}

// NewHorizontalController creates a new HorizontalController.
func NewHorizontalController(
	ctx context.Context,
	mgr ctrl.Manager,
	syncPeriod time.Duration,
	downscaleStabilisationWindow time.Duration,
	tolerance float64,
	cpuInitializationPeriod,
	delayOfInitialReadinessStatus time.Duration,
) (*HorizontalController, error) {
	clientConfig := mgr.GetConfig()
	clientSet, err := kube.NewClientSet(clientConfig)
	if err != nil {
		return nil, err
	}

	cacher := mgr.GetCache()
	podInformer, err := cacher.GetInformerForKind(ctx, corev1.SchemeGroupVersion.WithKind("Pod"))
	if err != nil {
		return nil, err
	}
	podLister := corelisters.NewPodLister(podInformer.(toolscache.SharedIndexInformer).GetIndexer())

	cs, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	apiVersionsGetter := customclient.NewAvailableAPIsGetter(cs.Discovery())
	// invalidate the discovery information roughly once per resync interval our API
	// information is *at most* two resync intervals old.
	go customclient.PeriodicallyInvalidate(
		apiVersionsGetter,
		syncPeriod,
		ctx.Done())

	clientConfig.Burst = 200
	clientConfig.QPS = 20
	discoveryClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	cachedClient := memory.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	go wait.Until(func() {
		restMapper.Reset()
	}, 30*time.Second, ctx.Done())

	metricsClient := metricsclient.NewRESTMetricsClient(
		resourceclient.NewForConfigOrDie(mgr.GetConfig()),
		customclient.NewForConfig(mgr.GetConfig(), restMapper, apiVersionsGetter),
		externalclient.NewForConfigOrDie(mgr.GetConfig()),
	)

	hpaController := &HorizontalController{
		client:                       mgr.GetClient(),
		clientSet:                    clientSet,
		eventRecorder:                mgr.GetEventRecorderFor(controllerName),
		downscaleStabilisationWindow: downscaleStabilisationWindow,
		podLister:                    podLister,
		recommendations:              map[string][]timestampedRecommendation{},
		recommendationsLock:          sync.Mutex{},
		scaleUpEvents:                map[string][]timestampedScaleEvent{},
		scaleUpEventsLock:            sync.RWMutex{},
		scaleDownEvents:              map[string][]timestampedScaleEvent{},
		scaleDownEventsLock:          sync.RWMutex{},
		hpaSelectors:                 selectors.NewBiMultimap(),
		hpaSelectorsMux:              sync.Mutex{},
	}

	replicaCalc := podautoscaler.NewReplicaCalculator(
		metricsClient,
		hpaController.podLister,
		tolerance,
		cpuInitializationPeriod,
		delayOfInitialReadinessStatus,
	)
	hpaController.replicaCalc = replicaCalc

	return hpaController, nil
}

// +kubebuilder:rbac:groups="metrics.k8s.io",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;list
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=autoscaling.nebula-graph.io,resources=nebulaautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling.nebula-graph.io,resources=nebulaautoscalers/status,verbs=get;update;patch

func (a *HorizontalController) Reconcile(ctx context.Context, request ctrl.Request) (res reconcile.Result, retErr error) {
	key := request.NamespacedName.String()
	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.Requeue || res.RequeueAfter > 0 {
				klog.Infof("Finished reconciling NebulaAutoscaler [%s] (%v), result: %v", key, time.Since(startTime), res)
			} else {
				klog.Infof("Finished reconciling NebulaAutoscaler [%s], spendTime: (%v)", key, time.Since(startTime))
			}
		} else {
			klog.Errorf("Failed to reconcile NebulaAutoscaler [%s], spendTime: (%v)", key, time.Since(startTime))
		}
	}()

	var hpa v1alpha1.NebulaAutoscaler
	if err := a.client.Get(ctx, request.NamespacedName, &hpa); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Skipping because NebulaAutoscaler [%s] has been deleted", key)
		}
		a.recommendationsLock.Lock()
		delete(a.recommendations, key)
		a.recommendationsLock.Unlock()

		a.scaleUpEventsLock.Lock()
		delete(a.scaleUpEvents, key)
		a.scaleUpEventsLock.Unlock()

		a.scaleDownEventsLock.Lock()
		delete(a.scaleDownEvents, key)
		a.scaleDownEventsLock.Unlock()
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := a.reconcileAutoscaler(ctx, &hpa, key); err != nil {
		if err == errNebulaClusterNotFound {
			klog.Infof("Skipping because NebulaCluster [%s] has been deleted", hpa.Spec.NebulaClusterRef.Name)
			return defaultResult(&hpa), nil
		}
		klog.Errorf("NebulaAutoscaler [%s] reconcile failed: %v", key, err)
		return ctrl.Result{RequeueAfter: defaultTimeout}, nil
	}

	return defaultResult(&hpa), nil
}

func (a *HorizontalController) reconcileAutoscaler(ctx context.Context, hpaShared *v1alpha1.NebulaAutoscaler, key string) (retErr error) {
	// make a copy so that we never mutate the shared informer cache (conversion can mutate the object)
	hpa := hpaShared.DeepCopy()
	hpaStatusOriginal := hpa.Status.DeepCopy()

	nc, err := a.clientSet.NebulaCluster().GetNebulaCluster(hpaShared.Namespace, hpaShared.Spec.NebulaClusterRef.Name)
	if err != nil {
		a.eventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedGetNebulaCluster", err.Error())
		setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionFalse, "FailedGetNebulaCluster", "the HPA controller unable to get the target cluster: %v", err)
		if err := a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
			utilruntime.HandleError(err)
		}
		if apierrors.IsNotFound(err) {
			a.setCurrentReplicasInStatus(hpa, 0)
			setCondition(hpa, v1alpha1.AutoscalerReady, corev1.ConditionFalse, "NebulaClusterReady", "the HPA controller was waiting for target cluster ready")
			if err := a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
				return err
			}
			return errNebulaClusterNotFound
		}
		return fmt.Errorf("unable to get target cluster: %v", err)
	}
	setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionTrue, "SucceededGetNebulaCluster", "the HPA controller was able to get the target cluster")
	currentReplicas := pointer.Int32Deref(nc.Spec.Graphd.Replicas, 0)
	a.recordInitialRecommendation(currentReplicas, key)

	var (
		metricStatuses        []autoscalingv2.MetricStatus
		metricDesiredReplicas int32
		metricName            string
	)

	desiredReplicas := int32(0)
	rescaleReason := ""

	var minReplicas int32

	if hpa.Spec.GraphdPolicy.MinReplicas != nil {
		minReplicas = *hpa.Spec.GraphdPolicy.MinReplicas
	} else {
		// Default value
		minReplicas = 1
	}

	rescale := true
	logger := klog.FromContext(ctx)

	if currentReplicas == 0 && minReplicas != 0 {
		// Autoscaling is disabled for this resource
		desiredReplicas = 0
		rescale = false
		setCondition(hpa, v1alpha1.AutoscalerActive, corev1.ConditionFalse, "ScalingDisabled", "scaling is disabled since the replica count of the target is zero")
	} else if currentReplicas > hpa.Spec.GraphdPolicy.MaxReplicas {
		rescaleReason = "Current number of replicas above Spec.MaxReplicas"
		desiredReplicas = hpa.Spec.GraphdPolicy.MaxReplicas
	} else if currentReplicas < minReplicas {
		rescaleReason = "Current number of replicas below Spec.MinReplicas"
		desiredReplicas = minReplicas
	} else {
		var metricTimestamp time.Time
		metricDesiredReplicas, metricName, metricStatuses, metricTimestamp, err = a.computeReplicasForMetrics(ctx, hpa, nc, hpa.Spec.GraphdPolicy.Metrics)
		// computeReplicasForMetrics may return both non-zero metricDesiredReplicas and an error.
		// That means some metrics still work and HPA should perform scaling based on them.
		if err != nil && metricDesiredReplicas == -1 {
			a.setCurrentReplicasInStatus(hpa, currentReplicas)
			if err := a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
				utilruntime.HandleError(err)
			}
			a.eventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedComputeMetricsReplicas", err.Error())
			return fmt.Errorf("failed to compute desired number of replicas based on listed metrics for %s: %v", hpa.Spec.NebulaClusterRef.Name, err)
		}
		if err != nil {
			// We proceed to scaling, but return this error from reconcileAutoscaler() finally.
			retErr = err
		}

		logger.V(4).Info("Proposing desired replicas",
			"desiredReplicas", metricDesiredReplicas,
			"metric", metricName,
			"timestamp", metricTimestamp,
			"scaleTarget", hpa.Spec.NebulaClusterRef.Name)

		rescaleMetric := ""
		if metricDesiredReplicas > desiredReplicas {
			desiredReplicas = metricDesiredReplicas
			rescaleMetric = metricName
		}
		if desiredReplicas > currentReplicas {
			rescaleReason = fmt.Sprintf("%s above target", rescaleMetric)
		}
		if desiredReplicas < currentReplicas {
			rescaleReason = "All metrics below target"
		}
		if hpa.Spec.GraphdPolicy.Behavior == nil {
			desiredReplicas = a.normalizeDesiredReplicas(hpa, key, currentReplicas, desiredReplicas, minReplicas)
		} else {
			desiredReplicas = a.normalizeDesiredReplicasWithBehaviors(hpa, key, currentReplicas, desiredReplicas, minReplicas)
		}
		rescale = desiredReplicas != currentReplicas
	}

	if rescale {
		nc.Spec.Graphd.Replicas = pointer.Int32(desiredReplicas)
		err = a.clientSet.NebulaCluster().UpdateNebulaCluster(nc)
		if err != nil {
			a.eventRecorder.Eventf(hpa, corev1.EventTypeWarning, "FailedRescale", "New size: %d; reason: %s; error: %v", desiredReplicas, rescaleReason, err.Error())
			setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionFalse, "FailedUpdateScale", "the HPA controller was unable to update the target cluster replicas: %v", err)
			a.setCurrentReplicasInStatus(hpa, currentReplicas)
			if err := a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
				utilruntime.HandleError(err)
			}
			return fmt.Errorf("failed to rescale %s: %v", hpa.Spec.NebulaClusterRef.Name, err)
		}
		setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionTrue, "SucceededRescale", "the HPA controller was able to update the target cluster replicas to %d", desiredReplicas)
		a.eventRecorder.Eventf(hpa, corev1.EventTypeNormal, "SuccessfulRescale", "New size: %d; reason: %s", desiredReplicas, rescaleReason)
		a.storeScaleEvent(hpa.Spec.GraphdPolicy.Behavior, key, currentReplicas, desiredReplicas)
		logger.Info("Successfully rescaled",
			"HPA", klog.KObj(hpa),
			"currentReplicas", currentReplicas,
			"desiredReplicas", desiredReplicas,
			"reason", rescaleReason)
	} else {
		logger.V(4).Info("Decided not to scale",
			"scaleTarget", hpa.Spec.NebulaClusterRef.Name,
			"desiredReplicas", desiredReplicas,
			"lastScaleTime", hpa.Status.GraphdStatus.LastScaleTime)
		desiredReplicas = currentReplicas
	}

	if !nc.IsConditionReady() {
		setCondition(hpa, v1alpha1.AutoscalerReady, corev1.ConditionFalse, "NebulaClusterReady", "the HPA controller was waiting for target cluster ready")
	} else {
		setCondition(hpa, v1alpha1.AutoscalerReady, corev1.ConditionTrue, "NebulaClusterReady", "the target cluster status is ready")
	}

	a.setStatus(hpa, currentReplicas, desiredReplicas, metricStatuses, rescale)

	if err := a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
		// we can overwrite retErr in this case because it's an internal error.
		return err
	}

	return retErr
}

// computeReplicasForMetrics computes the desired number of replicas for the metric specifications listed in the HPA,
// returning the maximum of the computed replica counts, a description of the associated metric, and the statuses of
// all metrics computed.
// It may return both valid metricDesiredReplicas and an error,
// when some metrics still work and HPA should perform scaling based on them.
// If HPA cannot do anything due to error, it returns -1 in metricDesiredReplicas as a failure signal.
func (a *HorizontalController) computeReplicasForMetrics(ctx context.Context, hpa *v1alpha1.NebulaAutoscaler, nc *appsv1alpha1.NebulaCluster,
	metricSpecs []autoscalingv2.MetricSpec) (replicas int32, metric string, statuses []autoscalingv2.MetricStatus, timestamp time.Time, err error) {

	ncSelector := nc.GraphdComponent().GenerateLabels()
	ncLabels := label.Label(ncSelector).Copy().Labels()
	selector, err := a.validateAndParseSelector(hpa, ncLabels.String())
	if err != nil {
		return -1, "", nil, time.Time{}, err
	}

	specReplicas := pointer.Int32Deref(nc.Spec.Graphd.Replicas, 0)
	statusReplicas := nc.Status.Graphd.Workload.Replicas
	statuses = make([]autoscalingv2.MetricStatus, len(metricSpecs))

	invalidMetricsCount := 0
	var invalidMetricError error
	var invalidMetricCondition v1alpha1.NebulaAutoscalerCondition

	for i, metricSpec := range metricSpecs {
		replicaCountProposal, metricNameProposal, timestampProposal, condition, err := a.computeReplicasForMetric(ctx, hpa, metricSpec, specReplicas, statusReplicas, selector, &statuses[i])

		if err != nil {
			if invalidMetricsCount <= 0 {
				invalidMetricCondition = condition
				invalidMetricError = err
			}
			invalidMetricsCount++
			continue
		}
		if replicas == 0 || replicaCountProposal > replicas {
			timestamp = timestampProposal
			replicas = replicaCountProposal
			metric = metricNameProposal
		}
	}

	if invalidMetricError != nil {
		invalidMetricError = fmt.Errorf("invalid metrics (%v invalid out of %v), first error is: %v", invalidMetricsCount, len(metricSpecs), invalidMetricError)
	}

	// If all metrics are invalid or some are invalid, and we would scale down,
	// return an error and set the condition of the hpa based on the first invalid metric.
	// Otherwise, set the condition as scaling active as we're going to scale
	if invalidMetricsCount >= len(metricSpecs) || (invalidMetricsCount > 0 && replicas < specReplicas) {
		setCondition(hpa, invalidMetricCondition.Type, invalidMetricCondition.Status, invalidMetricCondition.Reason, invalidMetricCondition.Message)
		return -1, "", statuses, time.Time{}, invalidMetricError
	}
	setCondition(hpa, v1alpha1.AutoscalerActive, corev1.ConditionTrue, "ValidMetricFound", "the HPA was able to successfully calculate a replica count from %s", metric)

	return replicas, metric, statuses, timestamp, invalidMetricError
}

// validateAndParseSelector verifies that:
// - selector is not empty;
// - selector format is valid;
// - all pods by current selector are controlled by only one HPA.
// Returns an error if the check has failed or the parsed selector if succeeded.
// In case of an error the ScalingActive is set to false with the corresponding reason.
func (a *HorizontalController) validateAndParseSelector(hpa *v1alpha1.NebulaAutoscaler, selector string) (labels.Selector, error) {
	if selector == "" {
		errMsg := "selector is required"
		a.eventRecorder.Event(hpa, corev1.EventTypeWarning, "SelectorRequired", errMsg)
		setCondition(hpa, v1alpha1.AutoscalerActive, corev1.ConditionFalse, "InvalidSelector", "the HPA target's scale is missing a selector")
		return nil, fmt.Errorf(errMsg)
	}

	parsedSelector, err := labels.Parse(selector)
	if err != nil {
		errMsg := fmt.Sprintf("couldn't convert selector into a corresponding internal selector object: %v", err)
		a.eventRecorder.Event(hpa, corev1.EventTypeWarning, "InvalidSelector", errMsg)
		setCondition(hpa, v1alpha1.AutoscalerActive, corev1.ConditionFalse, "InvalidSelector", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	hpaKey := selectors.Key{Name: hpa.Name, Namespace: hpa.Namespace}
	a.hpaSelectorsMux.Lock()
	if a.hpaSelectors.SelectorExists(hpaKey) {
		// Update HPA selector only if the HPA was registered in enqueueHPA.
		a.hpaSelectors.PutSelector(hpaKey, parsedSelector)
	}
	a.hpaSelectorsMux.Unlock()

	pods, err := a.podLister.Pods(hpa.Namespace).List(parsedSelector)
	if err != nil {
		return nil, err
	}

	selectingHpas := a.hpasControllingPodsUnderSelector(pods)
	if len(selectingHpas) > 1 {
		errMsg := fmt.Sprintf("pods by selector %v are controlled by multiple HPAs: %v", selector, selectingHpas)
		a.eventRecorder.Event(hpa, corev1.EventTypeWarning, "AmbiguousSelector", errMsg)
		setCondition(hpa, v1alpha1.AutoscalerActive, corev1.ConditionFalse, "AmbiguousSelector", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	return parsedSelector, nil
}

func (a *HorizontalController) recordInitialRecommendation(currentReplicas int32, key string) {
	a.recommendationsLock.Lock()
	defer a.recommendationsLock.Unlock()
	if a.recommendations[key] == nil {
		a.recommendations[key] = []timestampedRecommendation{{currentReplicas, time.Now()}}
	}
}

// hpasControllingPodsUnderSelector returns a list of keys of all HPAs that control a given list of pods.
func (a *HorizontalController) hpasControllingPodsUnderSelector(pods []*corev1.Pod) []selectors.Key {
	a.hpaSelectorsMux.Lock()
	defer a.hpaSelectorsMux.Unlock()

	hpas := map[selectors.Key]struct{}{}
	for _, p := range pods {
		podKey := selectors.Key{Name: p.Name, Namespace: p.Namespace}
		a.hpaSelectors.Put(podKey, p.Labels)

		selectingHpas, ok := a.hpaSelectors.ReverseSelect(podKey)
		if !ok {
			continue
		}
		for _, hpa := range selectingHpas {
			hpas[hpa] = struct{}{}
		}
	}
	// Clean up all added pods.
	a.hpaSelectors.KeepOnly([]selectors.Key{})

	var hpaList []selectors.Key
	for hpa := range hpas {
		hpaList = append(hpaList, hpa)
	}
	return hpaList
}

// stabilizeRecommendation:
// - replaces old recommendation with the newest recommendation,
// - returns max of recommendations that are not older than downscaleStabilisationWindow.
func (a *HorizontalController) stabilizeRecommendation(key string, prenormalizedDesiredReplicas int32) int32 {
	maxRecommendation := prenormalizedDesiredReplicas
	foundOldSample := false
	oldSampleIndex := 0
	cutoff := time.Now().Add(-a.downscaleStabilisationWindow)

	a.recommendationsLock.Lock()
	defer a.recommendationsLock.Unlock()
	for i, rec := range a.recommendations[key] {
		if rec.timestamp.Before(cutoff) {
			foundOldSample = true
			oldSampleIndex = i
		} else if rec.recommendation > maxRecommendation {
			maxRecommendation = rec.recommendation
		}
	}
	if foundOldSample {
		a.recommendations[key][oldSampleIndex] = timestampedRecommendation{prenormalizedDesiredReplicas, time.Now()}
	} else {
		a.recommendations[key] = append(a.recommendations[key], timestampedRecommendation{prenormalizedDesiredReplicas, time.Now()})
	}
	return maxRecommendation
}

// normalizeDesiredReplicas takes the metrics desired replicas value and normalizes it based on the appropriate conditions (i.e. < maxReplicas, >
// minReplicas, etc...)
func (a *HorizontalController) normalizeDesiredReplicas(hpa *v1alpha1.NebulaAutoscaler, key string, currentReplicas int32, prenormalizedDesiredReplicas int32, minReplicas int32) int32 {
	stabilizedRecommendation := a.stabilizeRecommendation(key, prenormalizedDesiredReplicas)
	if stabilizedRecommendation != prenormalizedDesiredReplicas {
		setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionTrue, "ScaleDownStabilized", "recent recommendations were higher than current one, applying the highest recent recommendation")
	} else {
		setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionTrue, "ReadyForNewScale", "recommended size matches current size")
	}

	desiredReplicas, condition, reason := convertDesiredReplicasWithRules(currentReplicas, stabilizedRecommendation, minReplicas, hpa.Spec.GraphdPolicy.MaxReplicas)

	if desiredReplicas == stabilizedRecommendation {
		setCondition(hpa, v1alpha1.AutoscalerLimited, corev1.ConditionFalse, condition, reason)
	} else {
		setCondition(hpa, v1alpha1.AutoscalerLimited, corev1.ConditionTrue, condition, reason)
	}

	return desiredReplicas
}

// NormalizationArg is used to pass all needed information between functions as one structure
type NormalizationArg struct {
	Key               string
	ScaleUpBehavior   *autoscalingv2.HPAScalingRules
	ScaleDownBehavior *autoscalingv2.HPAScalingRules
	MinReplicas       int32
	MaxReplicas       int32
	CurrentReplicas   int32
	DesiredReplicas   int32
}

// normalizeDesiredReplicasWithBehaviors takes the metrics desired replicas value and normalizes it:
// 1. Apply the basic conditions (i.e. < maxReplicas, > minReplicas, etc...)
// 2. Apply the scale up/down limits from the hpaSpec.Behaviors (i.e. add no more than 4 pods)
// 3. Apply the constraints period (i.e. add no more than 4 pods per minute)
// 4. Apply the stabilization (i.e. add no more than 4 pods per minute, and pick the smallest recommendation during last 5 minutes)
func (a *HorizontalController) normalizeDesiredReplicasWithBehaviors(hpa *v1alpha1.NebulaAutoscaler, key string, currentReplicas, prenormalizedDesiredReplicas, minReplicas int32) int32 {
	a.maybeInitScaleDownStabilizationWindow(hpa)
	normalizationArg := NormalizationArg{
		Key:               key,
		ScaleUpBehavior:   hpa.Spec.GraphdPolicy.Behavior.ScaleUp,
		ScaleDownBehavior: hpa.Spec.GraphdPolicy.Behavior.ScaleDown,
		MinReplicas:       minReplicas,
		MaxReplicas:       hpa.Spec.GraphdPolicy.MaxReplicas,
		CurrentReplicas:   currentReplicas,
		DesiredReplicas:   prenormalizedDesiredReplicas}
	stabilizedRecommendation, reason, message := a.stabilizeRecommendationWithBehaviors(normalizationArg)
	normalizationArg.DesiredReplicas = stabilizedRecommendation
	if stabilizedRecommendation != prenormalizedDesiredReplicas {
		// "ScaleUpStabilized" || "ScaleDownStabilized"
		setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionTrue, reason, message)
	} else {
		setCondition(hpa, v1alpha1.AbleToScale, corev1.ConditionTrue, "ReadyForNewScale", "recommended size matches current size")
	}
	desiredReplicas, reason, message := a.convertDesiredReplicasWithBehaviorRate(normalizationArg)
	if desiredReplicas == stabilizedRecommendation {
		setCondition(hpa, v1alpha1.AutoscalerLimited, corev1.ConditionFalse, reason, message)
	} else {
		setCondition(hpa, v1alpha1.AutoscalerLimited, corev1.ConditionTrue, reason, message)
	}

	return desiredReplicas
}

func (a *HorizontalController) maybeInitScaleDownStabilizationWindow(hpa *v1alpha1.NebulaAutoscaler) {
	behavior := hpa.Spec.GraphdPolicy.Behavior
	if behavior != nil && behavior.ScaleDown != nil && behavior.ScaleDown.StabilizationWindowSeconds == nil {
		stabilizationWindowSeconds := (int32)(a.downscaleStabilisationWindow.Seconds())
		hpa.Spec.GraphdPolicy.Behavior.ScaleDown.StabilizationWindowSeconds = &stabilizationWindowSeconds
	}
}

// getReplicasChangePerPeriod function find all the replica changes per period
func getReplicasChangePerPeriod(periodSeconds int32, scaleEvents []timestampedScaleEvent) int32 {
	period := time.Second * time.Duration(periodSeconds)
	cutoff := time.Now().Add(-period)
	var replicas int32
	for _, rec := range scaleEvents {
		if rec.timestamp.After(cutoff) {
			replicas += rec.replicaChange
		}
	}
	return replicas

}

func (a *HorizontalController) getUnableComputeReplicaCountCondition(hpa runtime.Object, reason string, err error) (condition v1alpha1.NebulaAutoscalerCondition) {
	a.eventRecorder.Event(hpa, corev1.EventTypeWarning, reason, err.Error())
	return v1alpha1.NebulaAutoscalerCondition{
		Type:    v1alpha1.AutoscalerActive,
		Status:  corev1.ConditionFalse,
		Reason:  reason,
		Message: fmt.Sprintf("the HPA was unable to compute the replica count: %v", err),
	}
}

// storeScaleEvent stores (adds or replaces outdated) scale event.
// outdated events to be replaced were marked as outdated in the `markScaleEventsOutdated` function
func (a *HorizontalController) storeScaleEvent(behavior *autoscalingv2.HorizontalPodAutoscalerBehavior, key string, prevReplicas, newReplicas int32) {
	if behavior == nil {
		return // we should not store any event as they will not be used
	}
	var oldSampleIndex int
	var longestPolicyPeriod int32
	foundOldSample := false
	if newReplicas > prevReplicas {
		longestPolicyPeriod = getLongestPolicyPeriod(behavior.ScaleUp)

		a.scaleUpEventsLock.Lock()
		defer a.scaleUpEventsLock.Unlock()
		markScaleEventsOutdated(a.scaleUpEvents[key], longestPolicyPeriod)
		replicaChange := newReplicas - prevReplicas
		for i, event := range a.scaleUpEvents[key] {
			if event.outdated {
				foundOldSample = true
				oldSampleIndex = i
			}
		}
		newEvent := timestampedScaleEvent{replicaChange, time.Now(), false}
		if foundOldSample {
			a.scaleUpEvents[key][oldSampleIndex] = newEvent
		} else {
			a.scaleUpEvents[key] = append(a.scaleUpEvents[key], newEvent)
		}
	} else {
		longestPolicyPeriod = getLongestPolicyPeriod(behavior.ScaleDown)

		a.scaleDownEventsLock.Lock()
		defer a.scaleDownEventsLock.Unlock()
		markScaleEventsOutdated(a.scaleDownEvents[key], longestPolicyPeriod)
		replicaChange := prevReplicas - newReplicas
		for i, event := range a.scaleDownEvents[key] {
			if event.outdated {
				foundOldSample = true
				oldSampleIndex = i
			}
		}
		newEvent := timestampedScaleEvent{replicaChange, time.Now(), false}
		if foundOldSample {
			a.scaleDownEvents[key][oldSampleIndex] = newEvent
		} else {
			a.scaleDownEvents[key] = append(a.scaleDownEvents[key], newEvent)
		}
	}
}

// stabilizeRecommendationWithBehaviors:
// - replaces old recommendation with the newest recommendation,
// - returns {max,min} of recommendations that are not older than constraints.Scale{Up,Down}.DelaySeconds
func (a *HorizontalController) stabilizeRecommendationWithBehaviors(args NormalizationArg) (int32, string, string) {
	now := time.Now()

	foundOldSample := false
	oldSampleIndex := 0

	upRecommendation := args.DesiredReplicas
	upDelaySeconds := *args.ScaleUpBehavior.StabilizationWindowSeconds
	upCutoff := now.Add(-time.Second * time.Duration(upDelaySeconds))

	downRecommendation := args.DesiredReplicas
	downDelaySeconds := *args.ScaleDownBehavior.StabilizationWindowSeconds
	downCutoff := now.Add(-time.Second * time.Duration(downDelaySeconds))

	// Calculate the upper and lower stabilization limits.
	a.recommendationsLock.Lock()
	defer a.recommendationsLock.Unlock()
	for i, rec := range a.recommendations[args.Key] {
		if rec.timestamp.After(upCutoff) {
			upRecommendation = min(rec.recommendation, upRecommendation)
		}
		if rec.timestamp.After(downCutoff) {
			downRecommendation = max(rec.recommendation, downRecommendation)
		}
		if rec.timestamp.Before(upCutoff) && rec.timestamp.Before(downCutoff) {
			foundOldSample = true
			oldSampleIndex = i
		}
	}

	// Bring the recommendation to within the upper and lower limits (stabilize).
	recommendation := args.CurrentReplicas
	if recommendation < upRecommendation {
		recommendation = upRecommendation
	}
	if recommendation > downRecommendation {
		recommendation = downRecommendation
	}

	// Record the unstabilized recommendation.
	if foundOldSample {
		a.recommendations[args.Key][oldSampleIndex] = timestampedRecommendation{args.DesiredReplicas, time.Now()}
	} else {
		a.recommendations[args.Key] = append(a.recommendations[args.Key], timestampedRecommendation{args.DesiredReplicas, time.Now()})
	}

	// Determine a human-friendly message.
	var reason, message string
	if args.DesiredReplicas >= args.CurrentReplicas {
		reason = "ScaleUpStabilized"
		message = "recent recommendations were lower than current one, applying the lowest recent recommendation"
	} else {
		reason = "ScaleDownStabilized"
		message = "recent recommendations were higher than current one, applying the highest recent recommendation"
	}
	return recommendation, reason, message
}

// convertDesiredReplicasWithBehaviorRate performs the actual normalization, given the constraint rate
// It doesn't consider the stabilizationWindow, it is done separately
func (a *HorizontalController) convertDesiredReplicasWithBehaviorRate(args NormalizationArg) (int32, string, string) {
	var possibleLimitingReason, possibleLimitingMessage string

	if args.DesiredReplicas > args.CurrentReplicas {
		a.scaleUpEventsLock.RLock()
		defer a.scaleUpEventsLock.RUnlock()
		a.scaleDownEventsLock.RLock()
		defer a.scaleDownEventsLock.RUnlock()
		scaleUpLimit := calculateScaleUpLimitWithScalingRules(args.CurrentReplicas, a.scaleUpEvents[args.Key], a.scaleDownEvents[args.Key], args.ScaleUpBehavior)

		if scaleUpLimit < args.CurrentReplicas {
			// We shouldn't scale up further until the scaleUpEvents will be cleaned up
			scaleUpLimit = args.CurrentReplicas
		}
		maximumAllowedReplicas := args.MaxReplicas
		if maximumAllowedReplicas > scaleUpLimit {
			maximumAllowedReplicas = scaleUpLimit
			possibleLimitingReason = "ScaleUpLimit"
			possibleLimitingMessage = "the desired replica count is increasing faster than the maximum scale rate"
		} else {
			possibleLimitingReason = "TooManyReplicas"
			possibleLimitingMessage = "the desired replica count is more than the maximum replica count"
		}
		if args.DesiredReplicas > maximumAllowedReplicas {
			return maximumAllowedReplicas, possibleLimitingReason, possibleLimitingMessage
		}
	} else if args.DesiredReplicas < args.CurrentReplicas {
		a.scaleUpEventsLock.RLock()
		defer a.scaleUpEventsLock.RUnlock()
		a.scaleDownEventsLock.RLock()
		defer a.scaleDownEventsLock.RUnlock()
		scaleDownLimit := calculateScaleDownLimitWithBehaviors(args.CurrentReplicas, a.scaleUpEvents[args.Key], a.scaleDownEvents[args.Key], args.ScaleDownBehavior)

		if scaleDownLimit > args.CurrentReplicas {
			// We shouldn't scale down further until the scaleDownEvents will be cleaned up
			scaleDownLimit = args.CurrentReplicas
		}
		minimumAllowedReplicas := args.MinReplicas
		if minimumAllowedReplicas < scaleDownLimit {
			minimumAllowedReplicas = scaleDownLimit
			possibleLimitingReason = "ScaleDownLimit"
			possibleLimitingMessage = "the desired replica count is decreasing faster than the maximum scale rate"
		} else {
			possibleLimitingMessage = "the desired replica count is less than the minimum replica count"
			possibleLimitingReason = "TooFewReplicas"
		}
		if args.DesiredReplicas < minimumAllowedReplicas {
			return minimumAllowedReplicas, possibleLimitingReason, possibleLimitingMessage
		}
	}
	return args.DesiredReplicas, "DesiredWithinRange", "the desired count is within the acceptable range"
}

// convertDesiredReplicas performs the actual normalization, without depending on `HorizontalController` or `NebulaAutoscaler`
func convertDesiredReplicasWithRules(currentReplicas, desiredReplicas, hpaMinReplicas, hpaMaxReplicas int32) (int32, string, string) {

	var minimumAllowedReplicas int32
	var maximumAllowedReplicas int32

	var possibleLimitingCondition string
	var possibleLimitingReason string

	minimumAllowedReplicas = hpaMinReplicas

	// Do not scale up too much to prevent incorrect rapid increase of the number of master replicas caused by
	// bogus CPU usage report from heapster/kubelet (like in issue #32304).
	scaleUpLimit := calculateScaleUpLimit(currentReplicas)

	if hpaMaxReplicas > scaleUpLimit {
		maximumAllowedReplicas = scaleUpLimit
		possibleLimitingCondition = "ScaleUpLimit"
		possibleLimitingReason = "the desired replica count is increasing faster than the maximum scale rate"
	} else {
		maximumAllowedReplicas = hpaMaxReplicas
		possibleLimitingCondition = "TooManyReplicas"
		possibleLimitingReason = "the desired replica count is more than the maximum replica count"
	}

	if desiredReplicas < minimumAllowedReplicas {
		possibleLimitingCondition = "TooFewReplicas"
		possibleLimitingReason = "the desired replica count is less than the minimum replica count"

		return minimumAllowedReplicas, possibleLimitingCondition, possibleLimitingReason
	} else if desiredReplicas > maximumAllowedReplicas {
		return maximumAllowedReplicas, possibleLimitingCondition, possibleLimitingReason
	}

	return desiredReplicas, "DesiredWithinRange", "the desired count is within the acceptable range"
}

func calculateScaleUpLimit(currentReplicas int32) int32 {
	return int32(math.Max(scaleUpLimitFactor*float64(currentReplicas), scaleUpLimitMinimum))
}

// markScaleEventsOutdated set 'outdated=true' flag for all scale events that are not used by any HPA object
func markScaleEventsOutdated(scaleEvents []timestampedScaleEvent, longestPolicyPeriod int32) {
	period := time.Second * time.Duration(longestPolicyPeriod)
	cutoff := time.Now().Add(-period)
	for i, event := range scaleEvents {
		if event.timestamp.Before(cutoff) {
			// outdated scale event are marked for later reuse
			scaleEvents[i].outdated = true
		}
	}
}

func getLongestPolicyPeriod(scalingRules *autoscalingv2.HPAScalingRules) int32 {
	var longestPolicyPeriod int32
	for _, policy := range scalingRules.Policies {
		if policy.PeriodSeconds > longestPolicyPeriod {
			longestPolicyPeriod = policy.PeriodSeconds
		}
	}
	return longestPolicyPeriod
}

// calculateScaleUpLimitWithScalingRules returns the maximum number of pods that could be added for the given HPAScalingRules
func calculateScaleUpLimitWithScalingRules(currentReplicas int32, scaleUpEvents, scaleDownEvents []timestampedScaleEvent, scalingRules *autoscalingv2.HPAScalingRules) int32 {
	var result int32
	var proposed int32
	var selectPolicyFn func(int32, int32) int32
	if *scalingRules.SelectPolicy == autoscalingv2.DisabledPolicySelect {
		return currentReplicas // Scaling is disabled
	} else if *scalingRules.SelectPolicy == autoscalingv2.MinChangePolicySelect {
		result = math.MaxInt32
		selectPolicyFn = min // For scaling up, the lowest change ('min' policy) produces a minimum value
	} else {
		result = math.MinInt32
		selectPolicyFn = max // Use the default policy otherwise to produce the highest possible change
	}
	for _, policy := range scalingRules.Policies {
		replicasAddedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleUpEvents)
		replicasDeletedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleDownEvents)
		periodStartReplicas := currentReplicas - replicasAddedInCurrentPeriod + replicasDeletedInCurrentPeriod
		if policy.Type == autoscalingv2.PodsScalingPolicy {
			proposed = periodStartReplicas + policy.Value
		} else if policy.Type == autoscalingv2.PercentScalingPolicy {
			// the proposal has to be rounded up because the proposed change might not increase the replica count causing the target to never scale up
			proposed = int32(math.Ceil(float64(periodStartReplicas) * (1 + float64(policy.Value)/100)))
		}
		result = selectPolicyFn(result, proposed)
	}
	return result
}

// calculateScaleDownLimitWithBehavior returns the maximum number of pods that could be deleted for the given HPAScalingRules
func calculateScaleDownLimitWithBehaviors(currentReplicas int32, scaleUpEvents, scaleDownEvents []timestampedScaleEvent, scalingRules *autoscalingv2.HPAScalingRules) int32 {
	var result int32
	var proposed int32
	var selectPolicyFn func(int32, int32) int32
	if *scalingRules.SelectPolicy == autoscalingv2.DisabledPolicySelect {
		return currentReplicas // Scaling is disabled
	} else if *scalingRules.SelectPolicy == autoscalingv2.MinChangePolicySelect {
		result = math.MinInt32
		selectPolicyFn = max // For scaling down, the lowest change ('min' policy) produces a maximum value
	} else {
		result = math.MaxInt32
		selectPolicyFn = min // Use the default policy otherwise to produce a highest possible change
	}
	for _, policy := range scalingRules.Policies {
		replicasAddedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleUpEvents)
		replicasDeletedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleDownEvents)
		periodStartReplicas := currentReplicas - replicasAddedInCurrentPeriod + replicasDeletedInCurrentPeriod
		if policy.Type == autoscalingv2.PodsScalingPolicy {
			proposed = periodStartReplicas - policy.Value
		} else if policy.Type == autoscalingv2.PercentScalingPolicy {
			proposed = int32(float64(periodStartReplicas) * (1 - float64(policy.Value)/100))
		}
		result = selectPolicyFn(result, proposed)
	}
	return result
}

// setCurrentReplicasInStatus sets the current replica count in the status of the HPA.
func (a *HorizontalController) setCurrentReplicasInStatus(hpa *v1alpha1.NebulaAutoscaler, currentReplicas int32) {
	a.setStatus(hpa, currentReplicas, hpa.Status.GraphdStatus.DesiredReplicas, hpa.Status.GraphdStatus.CurrentMetrics, false)
}

// setStatus recreates the status of the given HPA, updating the current and
// desired replicas, as well as the metric statuses
func (a *HorizontalController) setStatus(hpa *v1alpha1.NebulaAutoscaler, currentReplicas, desiredReplicas int32, metricStatuses []autoscalingv2.MetricStatus, rescale bool) {
	hpa.Status = v1alpha1.NebulaAutoscalerStatus{
		GraphdStatus: v1alpha1.AutoscalingPolicyStatus{
			CurrentReplicas: currentReplicas,
			DesiredReplicas: desiredReplicas,
			LastScaleTime:   hpa.Status.GraphdStatus.LastScaleTime,
			CurrentMetrics:  metricStatuses,
		},
		Conditions: hpa.Status.Conditions,
	}

	if rescale {
		now := metav1.NewTime(time.Now())
		hpa.Status.GraphdStatus.LastScaleTime = &now
	}
}

// updateStatusIfNeeded calls updateStatus only if the status of the new HPA is not the same as the old status
func (a *HorizontalController) updateStatusIfNeeded(ctx context.Context, oldStatus *v1alpha1.NebulaAutoscalerStatus, newHPA *v1alpha1.NebulaAutoscaler) error {
	if apiequality.Semantic.DeepEqual(oldStatus, &newHPA.Status) {
		return nil
	}
	return a.updateStatus(ctx, newHPA)
}

// updateStatus actually does the update request for the status of the given HPA
func (a *HorizontalController) updateStatus(ctx context.Context, hpa *v1alpha1.NebulaAutoscaler) error {
	err := a.clientSet.NebulaAutoscaler().UpdateNebulaAutoscalerStatus(hpa)
	if err != nil {
		a.eventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedUpdateStatus", err.Error())
		return fmt.Errorf("failed to update status for %s: %v", hpa.Name, err)
	}
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Successfully updated status", "HPA", klog.KObj(hpa))
	return nil
}

// setCondition sets the specific condition type on the given HPA to the specified value with the given reason
// and message.  The message and args are treated like a format string.  The condition will be added if it is
// not present.
func setCondition(hpa *v1alpha1.NebulaAutoscaler, conditionType v1alpha1.NebulaAutoscalerConditionType, status corev1.ConditionStatus, reason, message string, args ...interface{}) {
	hpa.Status.Conditions = setConditionInList(hpa.Status.Conditions, conditionType, status, reason, message, args...)
}

// setConditionInList sets the specific condition type on the given HPA to the specified value with the given
// reason and message.  The message and args are treated like a format string.  The condition will be added if
// it is not present.  The new list will be returned.
func setConditionInList(inputList []v1alpha1.NebulaAutoscalerCondition, conditionType v1alpha1.NebulaAutoscalerConditionType, status corev1.ConditionStatus, reason, message string, args ...interface{}) []v1alpha1.NebulaAutoscalerCondition {
	resList := inputList
	var existingCond *v1alpha1.NebulaAutoscalerCondition
	for i, condition := range resList {
		if condition.Type == conditionType {
			// can't take a pointer to an iteration variable
			existingCond = &resList[i]
			break
		}
	}

	if existingCond == nil {
		resList = append(resList, v1alpha1.NebulaAutoscalerCondition{
			Type: conditionType,
		})
		existingCond = &resList[len(resList)-1]
	}

	if existingCond.Status != status {
		existingCond.LastTransitionTime = metav1.Now()
	}

	existingCond.Status = status
	existingCond.Reason = reason
	existingCond.Message = fmt.Sprintf(message, args...)

	return resList
}

func max(a, b int32) int32 {
	if a >= b {
		return a
	}
	return b
}

func min(a, b int32) int32 {
	if a <= b {
		return a
	}
	return b
}

func defaultResult(hpa *v1alpha1.NebulaAutoscaler) reconcile.Result {
	requeueAfter := v1alpha1.DefaultPollingPeriod
	pollingPeriod := hpa.GetPollingPeriod()
	if pollingPeriod != nil {
		requeueAfter = pollingPeriod.Duration
	}
	return reconcile.Result{RequeueAfter: requeueAfter}
}

// SetupWithManager sets up the controller with the Manager.
func (a *HorizontalController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NebulaAutoscaler{}).
		Watches(&appsv1alpha1.NebulaCluster{}, handler.Funcs{}).
		Complete(a)
}
