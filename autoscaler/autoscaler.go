package autoscaler

import (
	"context"
	"fmt"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/util/ec"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/clock"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticcloud"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticsearch"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/memory"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/timeutil"
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

	// We'll apply in the below order not to cause unassigned shards issue
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
	minTopologySize, maxTopologySize, err := calcMinMaxTopologySize(a.config.Scaling)
	if err != nil {
		return nil, fmt.Errorf("calculate min miax topology size: %w", err)
	}

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

	if isWithinCoolDownPeriod, err := a.isWithinCoolDownPeriod(esResource.Info.PlanInfo); isWithinCoolDownPeriod {
		scalingOperation.Reason = "Within cool down period"
		// TODO: we may need to check if replica update needed
		return scalingOperation, nil
	} else if err != nil {
		return nil, fmt.Errorf("check if within cool down period: %w", err)
	}

	desiredTopologySize := currentTopology.Size
	desiredReplicaNum := indexSettings.ReplicaNum
	if autoScalingConfig := a.config.Scaling.AutoScaling; autoScalingConfig != nil {
		now := clock.Now()
		dataContentNodes := nodeStats.DataContentNodes()
		currentCPUUtil := dataContentNodes.AvgCPUUtil()

		fetchMetricsAfter := now.Add(-timeutil.MaxDuration(autoScalingConfig.ScaleOutThresholdDuration, autoScalingConfig.ScaleInThresholdDuration))
		cpuUtils, err := autoScalingConfig.MetricsProvider.GetCPUUtilMetrics(ctx, dataContentNodes.IDs(), fetchMetricsAfter)
		if err != nil {
			return nil, fmt.Errorf("get cpu util metrics: %w", err)
		}

		if cpuUtils.After(now.Add(-autoScalingConfig.ScaleOutThresholdDuration)).AllGreaterThan(float64(autoScalingConfig.DesiredCPUUtilPercent)) {
			// Consider scaling up
			maxNodeNum := elasticcloud.CalcTopologyNodeNum(&models.ElasticsearchClusterTopologyElement{
				Size:      maxTopologySize,
				ZoneCount: currentTopology.ZoneCount,
			})
			maxReplicaNum := maxNodeNum*a.config.Scaling.ShardsPerNode/indexSettings.ShardNum - 1

			minDiffFromDesiredCPUUtil := math.Abs(float64(autoScalingConfig.DesiredCPUUtilPercent) - currentCPUUtil)
			for replicaNum := indexSettings.ReplicaNum; replicaNum <= maxReplicaNum; replicaNum++ {
				nodeNum := elasticsearch.CalcTotalShardNum(indexSettings.ShardNum, replicaNum) / a.config.Scaling.ShardsPerNode
				estimatedCPUUtil := dataContentNodes.TotalCPUUtil() / float64(nodeNum)
				diffFromDesiredCPUUtil := math.Abs(float64(autoScalingConfig.DesiredCPUUtilPercent) - estimatedCPUUtil)

				if diffFromDesiredCPUUtil < minDiffFromDesiredCPUUtil {
					desiredTopologySize = elasticcloud.NewTopologySize(int32(nodeNum * 64))
					desiredReplicaNum = replicaNum
					minDiffFromDesiredCPUUtil = diffFromDesiredCPUUtil
				}
			}
		} else if cpuUtils.After(now.Add(-autoScalingConfig.ScaleInThresholdDuration)).AllLessThan(float64(autoScalingConfig.DesiredCPUUtilPercent)) {
			// Consider scaling down
			minNodeNum := elasticcloud.CalcTopologyNodeNum(&models.ElasticsearchClusterTopologyElement{
				Size:      minTopologySize,
				ZoneCount: currentTopology.ZoneCount,
			})
			minReplicaNum := minNodeNum*a.config.Scaling.ShardsPerNode/indexSettings.ShardNum - 1

			minDiffFromDesiredCPUUtil := math.Abs(float64(autoScalingConfig.DesiredCPUUtilPercent) - currentCPUUtil)
			for replicaNum := indexSettings.ReplicaNum; replicaNum >= minReplicaNum; replicaNum-- {
				nodeNum := elasticsearch.CalcTotalShardNum(indexSettings.ShardNum, replicaNum) / a.config.Scaling.ShardsPerNode
				estimatedCPUUtil := dataContentNodes.TotalCPUUtil() / float64(nodeNum)
				diffFromDesiredCPUUtil := math.Abs(float64(autoScalingConfig.DesiredCPUUtilPercent) - estimatedCPUUtil)

				if diffFromDesiredCPUUtil < minDiffFromDesiredCPUUtil {
					desiredTopologySize = elasticcloud.NewTopologySize(int32(nodeNum * 64))
					desiredReplicaNum = replicaNum
					minDiffFromDesiredCPUUtil = diffFromDesiredCPUUtil
				}
			}
		}
		scalingOperation.ToTopologySize = desiredTopologySize
		scalingOperation.ToReplicaNum = desiredReplicaNum
	}

	// TODO: Need to check replica count and min/max shard count
	if *desiredTopologySize.Value > *maxTopologySize.Value {
		scalingOperation.ToTopologySize = maxTopologySize
		scalingOperation.Reason = fmt.Sprintf(
			"desired topology size '%d' is greater than max topology size '%d'",
			*desiredTopologySize.Value,
			*maxTopologySize.Value,
		)
		return scalingOperation, nil
	}
	if *desiredTopologySize.Value < *minTopologySize.Value {
		scalingOperation.ToTopologySize = minTopologySize
		scalingOperation.Reason = fmt.Sprintf(
			"desired topology size '%d' is less than min topology size '%d'",
			*desiredTopologySize.Value,
			*minTopologySize.Value,
		)
		return scalingOperation, nil
	}
	return scalingOperation, nil
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

	minTopologySize := &models.TopologySize{
		Resource: ec.String("memory"),
		Value:    ec.Int32(memory.ConvertGiBToMiB(int32(minSizeMemoryGB))),
	}
	maxTopologySize := &models.TopologySize{
		Resource: ec.String("memory"),
		Value:    ec.Int32(memory.ConvertGiBToMiB(int32(maxSizeMemoryGB))),
	}
	return minTopologySize, maxTopologySize, nil
}
