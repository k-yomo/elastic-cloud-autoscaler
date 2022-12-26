package autoscaler

import (
	"context"
	"fmt"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/clock"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticcloud"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticsearch"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/memory"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/timeutil"
	"github.com/k0kubun/pp/v3"
	"github.com/robfig/cron/v3"
	"golang.org/x/sync/errgroup"
	"math"
	"time"
)

type AutoScalar struct {
	config *Config

	ecClient elasticcloud.Client
	esClient elasticsearch.Client
}

func New(config *Config) (*AutoScalar, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	ecClient := elasticcloud.NewClient(config.ElasticCloudClient, config.DeploymentID)
	esClient := elasticsearch.NewClient(config.ElasticsearchClient)
	return &AutoScalar{
		config:   config,
		ecClient: ecClient,
		esClient: esClient,
	}, nil
}

// Run executes scale in/out if needed based on the configuration
// This method will return non-nil ScalingOperation with error when error happens after scaling operation is decided
func (a *AutoScalar) Run(ctx context.Context) (*ScalingOperation, error) {
	scalingOperation, err := a.CalcScalingOperation(ctx)
	if err != nil {
		return nil, fmt.Errorf("calculate scaling operation: %w", err)
	}

	if a.config.DryRun {
		return scalingOperation, nil
	}

	// We'll apply updates in the following order not to cause unassigned shards issue
	// - replica scale-in => node scale-in when scaling in
	// - node scale-out => replica scale-out when scaling out
	if scalingOperation.FromReplicaNum > scalingOperation.ToReplicaNum {
		if err := a.esClient.UpdateIndexReplicaNum(ctx, a.config.Scaling.Index, scalingOperation.ToReplicaNum); err != nil {
			return scalingOperation, fmt.Errorf("update number of replicas: %w", err)
		}
	}
	if scalingOperation.NeedTopologySizeUpdate() {
		if err := a.ecClient.UpdateESHotContentTopologySize(ctx, scalingOperation.ToTopologySize); err != nil {
			return scalingOperation, fmt.Errorf("update topology size: %w", err)
		}
	}
	if scalingOperation.FromReplicaNum < scalingOperation.ToReplicaNum {
		if err := a.esClient.UpdateIndexReplicaNum(ctx, a.config.Scaling.Index, scalingOperation.ToReplicaNum); err != nil {
			return scalingOperation, fmt.Errorf("update number of replicas: %w", err)
		}
	}
	return scalingOperation, nil
}

func (a *AutoScalar) CalcScalingOperation(ctx context.Context) (*ScalingOperation, error) {
	esResource, nodeStats, indexSettings, err := a.getDataForDecidingScalingOperation(ctx)
	if err != nil {
		return nil, fmt.Errorf("get required data for deciding scaling operation: %w", err)
	}

	currentTopology := elasticcloud.FindHotContentTopology(esResource.Info.PlanInfo.Current.Plan.ClusterTopology)
	scalingOperation := &ScalingOperation{
		FromTopologySize: currentTopology.Size,
		ToTopologySize:   currentTopology.Size,
		FromReplicaNum:   indexSettings.ReplicaNum,
		ToReplicaNum:     indexSettings.ReplicaNum,
	}

	if esResource.Info.PlanInfo.Pending != nil {
		scalingOperation.Reason = "Pending plan exists"
		return scalingOperation, nil
	}

	minTopologySize, maxTopologySize, err := calcMinMaxTopologySize(a.config.Scaling)
	if err != nil {
		return nil, fmt.Errorf("calculate min miax topology size: %w", err)
	}
	availableTopologySizes, err := calcAvailableTopologySizes(
		minTopologySize,
		maxTopologySize,
		currentTopology.ZoneCount,
		a.config.Scaling.ShardsPerNode,
		indexSettings.ShardNum,
	)
	if err != nil {
		return nil, fmt.Errorf("calculate available topology sizes: %w", err)
	}

	if len(availableTopologySizes) == 0 {
		scalingOperation.Reason = "No possible topology size with the given configuration"
		return scalingOperation, nil
	}

	desiredTopologySize := currentTopology.Size
	if autoScalingConfig := a.config.Scaling.AutoScaling; autoScalingConfig != nil {
		isWithinCoolDownPeriod, err := a.isWithinCoolDownPeriod(esResource.Info.PlanInfo)
		if err != nil {
			return nil, fmt.Errorf("check if within cool down period: %w", err)
		}

		if !isWithinCoolDownPeriod {
			now := clock.Now()
			dataContentNodes := nodeStats.DataContentNodes()
			currentCPUUtil := dataContentNodes.AvgCPUUtil()

			fetchMetricsAfter := now.Add(-timeutil.MaxDuration(autoScalingConfig.ScaleOutThresholdDuration, autoScalingConfig.ScaleInThresholdDuration))
			cpuUtils, err := autoScalingConfig.MetricsProvider.GetCPUUtilMetrics(ctx, dataContentNodes.IDs(), fetchMetricsAfter)
			if err != nil {
				return nil, fmt.Errorf("get cpu util metrics: %w", err)
			}

			shouldScaleOut := cpuUtils.After(now.Add(-autoScalingConfig.ScaleOutThresholdDuration)).AllGreaterThan(float64(autoScalingConfig.DesiredCPUUtilPercent))
			shouldScaleIn := cpuUtils.After(now.Add(-autoScalingConfig.ScaleInThresholdDuration)).AllLessThan(float64(autoScalingConfig.DesiredCPUUtilPercent))
			if shouldScaleOut || shouldScaleIn {
				minDiffFromDesiredCPUUtil := math.Abs(float64(autoScalingConfig.DesiredCPUUtilPercent) - currentCPUUtil)
				pp.Println("currentDiffFromDesiredCPUUtil", minDiffFromDesiredCPUUtil)
				for _, topologySize := range availableTopologySizes {
					nodeNum := elasticcloud.CalcNodeNum(topologySize, currentTopology.ZoneCount)
					estimatedCPUUtil := dataContentNodes.TotalCPUUtil() / float64(nodeNum)
					diffFromDesiredCPUUtil := math.Abs(float64(autoScalingConfig.DesiredCPUUtilPercent) - estimatedCPUUtil)
					pp.Println("nodeNum", nodeNum)
					pp.Println("estimatedCPUUtil", estimatedCPUUtil)
					pp.Println("diffFromDesiredCPUUtil", diffFromDesiredCPUUtil)
					if diffFromDesiredCPUUtil < minDiffFromDesiredCPUUtil {
						desiredTopologySize = topologySize
						minDiffFromDesiredCPUUtil = diffFromDesiredCPUUtil
					}
				}
			}
			if *desiredTopologySize.Value != *currentTopology.Size.Value {
				scalingOperation.ToTopologySize = desiredTopologySize
				scalingOperation.ToReplicaNum = calcReplicaNumFromNodeNum(
					elasticcloud.CalcNodeNum(desiredTopologySize, currentTopology.ZoneCount),
					a.config.Scaling.ShardsPerNode,
					indexSettings.ShardNum,
				)
				if shouldScaleOut {
					scalingOperation.Reason = fmt.Sprintf("CPU utilization is greater than the desired CPU utilization '%d%%' for %.0f seconds", autoScalingConfig.DesiredCPUUtilPercent, autoScalingConfig.ScaleOutThresholdDuration.Seconds())
				} else if shouldScaleIn {
					scalingOperation.Reason = fmt.Sprintf("CPU utilization is less than the desired CPU utilization '%d%%' for %.0f seconds", autoScalingConfig.DesiredCPUUtilPercent, autoScalingConfig.ScaleInThresholdDuration.Seconds())
				}
			}
		}
	}

	if *desiredTopologySize.Value < *minTopologySize.Value {
		minAvailableTopologySize := availableTopologySizes[0]
		scalingOperation.ToTopologySize = minAvailableTopologySize
		scalingOperation.ToReplicaNum = calcReplicaNumFromNodeNum(
			elasticcloud.CalcNodeNum(minAvailableTopologySize, currentTopology.ZoneCount),
			a.config.Scaling.ShardsPerNode,
			indexSettings.ShardNum,
		)
		scalingOperation.Reason = fmt.Sprintf(
			"current or desired topology size '%dg' is less than min topology size '%dg'",
			memory.ConvertMibToGiB(*desiredTopologySize.Value),
			memory.ConvertMibToGiB(*minTopologySize.Value),
		)
		return scalingOperation, nil
	}
	if *desiredTopologySize.Value > *maxTopologySize.Value {
		maxAvailableTopologySize := availableTopologySizes[len(availableTopologySizes)-1]
		scalingOperation.ToTopologySize = maxAvailableTopologySize
		scalingOperation.ToReplicaNum = calcReplicaNumFromNodeNum(
			elasticcloud.CalcNodeNum(maxAvailableTopologySize, currentTopology.ZoneCount),
			a.config.Scaling.ShardsPerNode,
			indexSettings.ShardNum,
		)
		scalingOperation.Reason = fmt.Sprintf(
			"current or desired topology size '%dg' is greater than max topology size '%dg'",
			memory.ConvertMibToGiB(*desiredTopologySize.Value),
			memory.ConvertMibToGiB(*maxTopologySize.Value),
		)
		return scalingOperation, nil
	}
	return scalingOperation, nil
}

// calcAvailableTopologySizes calculates available topology sizes within given min/max sizes
// Only topology size that meets shardsPerZone will be returned
func calcAvailableTopologySizes(
	minTopologySize *models.TopologySize,
	maxTopologySize *models.TopologySize,
	zoneCount int32,
	shardsPerNode int,
	shardNum int,
) ([]*models.TopologySize, error) {
	var availableTopologySizes []*models.TopologySize

	minNodeNum := elasticcloud.CalcNodeNum(minTopologySize, zoneCount)
	maxNodeNum := elasticcloud.CalcNodeNum(maxTopologySize, zoneCount)
	minReplicaNum := calcReplicaNumFromNodeNum(minNodeNum, shardsPerNode, shardNum)
	maxReplicaNum := calcReplicaNumFromNodeNum(maxNodeNum, shardsPerNode, shardNum)
	for replicaNum := minReplicaNum; replicaNum <= maxReplicaNum; replicaNum++ {
		totalShardNum := elasticsearch.CalcTotalShardNum(shardNum, replicaNum)
		if totalShardNum%shardsPerNode != 0 {
			continue
		}
		nodeNum := totalShardNum / shardsPerNode
		if nodeNum%int(zoneCount) != 0 {
			continue
		}
		// TODO: Fix hardcoded 64 to support the other node sizes
		availableTopologySizes = append(availableTopologySizes, elasticcloud.NewTopologySize(int32(nodeNum/int(zoneCount)*64)))
	}
	return availableTopologySizes, nil
}

func (a *AutoScalar) getDataForDecidingScalingOperation(ctx context.Context) (
	*models.ElasticsearchResourceInfo,
	*elasticsearch.NodeStats,
	*elasticsearch.IndexSettings,
	error,
) {
	eg := errgroup.Group{}
	var esResource *models.ElasticsearchResourceInfo
	eg.Go(func() error {
		var err error
		esResource, err = a.ecClient.GetESResourceInfo(ctx, true)
		if err != nil {
			return fmt.Errorf("fetch elasticsearch resource info: %w", err)
		}
		return nil
	})

	var nodeStats *elasticsearch.NodeStats
	eg.Go(func() error {
		var err error
		nodeStats, err = a.esClient.GetNodeStats(ctx)
		if err != nil {
			return fmt.Errorf("fetch node stats: %w", err)
		}
		return nil
	})

	var indexSettings *elasticsearch.IndexSettings
	eg.Go(func() error {
		var err error
		indexSettings, err = a.esClient.GetIndexSettings(ctx, a.config.Scaling.Index)
		if err != nil {
			return fmt.Errorf("fetch node stats: %w", err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, err
	}
	return esResource, nodeStats, indexSettings, nil
}

func (a *AutoScalar) isWithinCoolDownPeriod(planInfo *models.ElasticsearchClusterPlansInfo) (bool, error) {
	currentTopology := elasticcloud.FindHotContentTopology(planInfo.Current.Plan.ClusterTopology)

	var scaledUp bool
	var lastSizeUpdatedTime time.Time
	// plan history is order by oldest
	for i := len(planInfo.History) - 1; i >= 0; i-- {
		topology := elasticcloud.FindHotContentTopology(planInfo.History[i].Plan.ClusterTopology)
		if *currentTopology.Size.Value == *topology.Size.Value {
			continue
		} else if *currentTopology.Size.Value > *topology.Size.Value {
			scaledUp = true
		}
		lastSizeUpdatedAt, err := time.Parse(timeutil.RFC3339Milli, planInfo.History[i].AttemptEndTime.String())
		if err != nil {
			return false, fmt.Errorf("parse last size updated time '%s': %w", planInfo.History[i].AttemptEndTime.String(), err)
		}
		lastSizeUpdatedTime = lastSizeUpdatedAt
		break
	}

	if lastSizeUpdatedTime.IsZero() {
		return false, nil
	}

	now := clock.Now()
	if scaledUp {
		return now.Before(lastSizeUpdatedTime.Add(time.Second * a.config.Scaling.AutoScaling.ScaleOutCoolDownDuration)), nil
	} else {
		return now.Before(lastSizeUpdatedTime.Add(time.Second * a.config.Scaling.AutoScaling.ScaleInCoolDownDuration)), nil
	}
}

func calcMinMaxTopologySize(config ScalingConfig) (min *models.TopologySize, max *models.TopologySize, err error) {
	now := clock.Now()
	minSizeMemoryGB := config.DefaultMinSizeMemoryGB
	maxSizeMemoryGB := config.DefaultMaxSizeMemoryGB
	for _, scheduledScaling := range config.ScheduledScalings {
		schedule, err := cron.ParseStandard(scheduledScaling.StartCronSchedule)
		if err != nil {
			return nil, nil, fmt.Errorf("parse cron scaling schedule: %w", err)
		}
		scheduledAt := schedule.Next(now.Add(-scheduledScaling.Duration))
		if scheduledAt.Before(now) && scheduledAt.Add(scheduledScaling.Duration).After(now) {
			if scheduledScaling.MinSizeMemoryGB > 0 {
				minSizeMemoryGB = scheduledScaling.MinSizeMemoryGB
			}
			if scheduledScaling.MinSizeMemoryGB > 0 {
				maxSizeMemoryGB = scheduledScaling.MaxSizeMemoryGB
			}
		}
	}

	return elasticcloud.NewTopologySize(int32(minSizeMemoryGB)),
		elasticcloud.NewTopologySize(int32(maxSizeMemoryGB)),
		nil
}

func calcReplicaNumFromNodeNum(nodeNum int, shardsPerNode int, shardNum int) int {
	return nodeNum*shardsPerNode/shardNum - 1
}
