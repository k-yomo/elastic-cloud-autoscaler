package autoscaler

import (
	"errors"
	"fmt"
	"time"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-playground/validator/v10"
	"github.com/k-yomo/elastic-cloud-autoscaler/metrics"
	"github.com/robfig/cron/v3"
)

type Config struct {
	ElasticCloudClient  *api.API                   `validate:"required"`
	DeploymentID        string                     `validate:"required"`
	ElasticsearchClient *elasticsearch.TypedClient `validate:"required"`

	Scaling ScalingConfig `validate:"required"`
	// DryRun disables applying actual scaling operation
	DryRun bool
}

type ScalingConfig struct {
	// Default memory min size. It can be overwritten by ScheduledScalingConfig.MinMemoryGBPerZone.
	// Available number is only 64,...(64xN node)
	// If you have multiple zone, this memory is per zone.
	// NOTE: Currently we only support 64GB node for simplicity
	DefaultMinMemoryGBPerZone int `validate:"gte=64"`
	// Default memory max size. It can be overwritten by ScheduledScalingConfig.MaxMemoryGBPerZone.
	// Available number is only 64,...(64xN node)
	// If you have multiple zone, this memory is per zone.
	// NOTE: Currently we only support 64GB node for simplicity
	DefaultMaxMemoryGBPerZone int `validate:"gte=64,gtefield=DefaultMinMemoryGBPerZone"`

	AutoScaling *AutoScalingConfig
	// If the time is within multiple schedules, the last schedule will be applied
	// e.g. [{min: 1, max: 2}, {min:2, max:4}] => min: 2, max: 4
	ScheduledScalings []*ScheduledScalingConfig

	// Index to update replicas when scaling out/in
	Index         string `validate:"required"`
	ShardsPerNode int    `validate:"required,gte=1"`
}

type ScalingThresholdDurationMinute int

type AutoScalingConfig struct {
	MetricsProvider       metrics.Provider `validate:"required"`
	DesiredCPUUtilPercent int              `validate:"required,gt=0,lt=100"`

	ScaleOutThresholdDuration time.Duration `validate:"gte=0"`
	ScaleOutCoolDownDuration  time.Duration `validate:"gte=0"`

	ScaleInThresholdDuration time.Duration `validate:"gte=0"`
	ScaleInCoolDownDuration  time.Duration `validate:"gte=0"`
}

// ScheduledScalingConfig represents scheduled min/max memory max size within the period
type ScheduledScalingConfig struct {
	// MinMemoryGBPerZone is the minimum memory size during the specified period
	// If 0, then `ScalingConfig.DefaultMinMemoryGBPerZone` will be used
	// NOTE: Currently we only support 64GB node for simplicity
	MinMemoryGBPerZone int `validate:"gte=64"`
	// MaxMemoryGBPerZone is the maximum memory size during the specified period
	// If 0, then `ScalingConfig.DefaultMaxMemoryGBPerZone` will be used
	// NOTE: Currently we only support 64GB node for simplicity
	MaxMemoryGBPerZone int `validate:"gte=64,gtefield=MinMemoryGBPerZone"`
	// cron format schedule
	// default timezone is machine local timezone,
	// if you want to specify, set TZ= prefix (e.g. `TZ=UTC 0 0 0 0 0`)
	StartCronSchedule string
	Duration          time.Duration `validate:"gt=0"`
}

func validateConfig(config *Config) error {
	if config == nil {
		return errors.New("config must not be nil")
	}
	if err := validator.New().Struct(config); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	for _, schedule := range config.Scaling.ScheduledScalings {
		if _, err := cron.ParseStandard(schedule.StartCronSchedule); err != nil {
			return fmt.Errorf("validate cron schedule: %w", err)
		}
	}
	return nil
}
