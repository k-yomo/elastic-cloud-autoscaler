package metrics

import (
	"context"
	"time"
)

//go:generate mockgen -source=$GOFILE -package=mock_$GOPACKAGE -destination=../mocks/$GOPACKAGE/mock_$GOFILE
type Provider interface {
	GetCPUUtilMetrics(ctx context.Context, targetNodeNames []string, after time.Time) (AvgCPUUtils, error)
}
