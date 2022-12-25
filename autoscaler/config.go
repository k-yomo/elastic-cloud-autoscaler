package autoscaler

import (
	"errors"
	"fmt"
	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-playground/validator/v10"
	"github.com/k-yomo/elastic-cloud-autoscaler/metrics"
	"github.com/robfig/cron/v3"
	"time"
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
	// Default memory min size. It can be overwritten by ScheduledScalingConfig.MinSizeMemoryGB.
	// Available number is only 1,2,4,8,16,34,64,...(64xN node)
	DefaultMinSizeMemoryGB int `validate:"gt=0"`
	// Default memory max size. It can be overwritten by ScheduledScalingConfig.MaxSizeMemoryGB.
	// Available number is only 1,2,4,8,16,34,64,...(64xN node)
	DefaultMaxSizeMemoryGB int `validate:"gt=0,gtefield=DefaultMinSizeMemoryGB"`

	AutoScaling       *AutoScalingConfig
	ScheduledScalings []*ScheduledScalingConfig

	// Index to update replicas when scaling out/in
	Index         string `validate:"required"`
	ShardsPerNode int    `validate:"required,gte=1"`
}

type ScalingThresholdDurationMinute int

const (
	ScalingThresholdDuration1Minute  = 1
	ScalingThresholdDuration5Minute  = 5
	ScalingThresholdDuration15Minute = 15
)

type AutoScalingConfig struct {
	MetricsProvider       metrics.Provider `validate:"required"`
	DesiredCPUUtilPercent int              `validate:"required,gt=0,lt=100"`

	ScaleOutThresholdDuration time.Duration `validate:"gte=0"`
	ScaleOutCoolDownDuration  time.Duration `validate:"gte=0"`

	ScaleInThresholdDuration time.Duration `validate:"gte=0"`
	ScaleInCoolDownDuration  time.Duration `validate:"gte=0"`
}

type ScheduledScalingConfig struct {
	// MinSizeMemoryGB is the minimum memory size during the specified period
	// If 0, then `ScalingConfig.DefaultMinSizeMemoryGB` will be used
	MinSizeMemoryGB int `validate:"gte=0"`
	// MaxSizeMemoryGB is the maximum memory size during the specified period
	// If 0, then `ScalingConfig.DefaultMaxSizeMemoryGB` will be used
	MaxSizeMemoryGB int `validate:"gte=0,gtefield=MinSizeMemoryGB"`
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
