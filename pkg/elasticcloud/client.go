package elasticcloud

import (
	"context"
	"errors"
	"fmt"
	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/client/deployments"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/plan"
	"github.com/elastic/cloud-sdk-go/pkg/plan/planutil"
	"github.com/elastic/cloud-sdk-go/pkg/util/ec"
	"time"
)

type Client interface {
	GetESResourceInfo(ctx context.Context, includePlanHistory bool) (*models.ElasticsearchResourceInfo, error)
	UpdateESHotContentTopologySize(ctx context.Context, topology *models.TopologySize) error
}

type clientImpl struct {
	ecAPI        *api.API
	deploymentID string
}

func NewClient(ecClient *api.API, deploymentID string) Client {
	return &clientImpl{
		ecAPI:        ecClient,
		deploymentID: deploymentID,
	}
}

func (c *clientImpl) GetESResourceInfo(ctx context.Context, includePlanHistory bool) (*models.ElasticsearchResourceInfo, error) {
	params := &deployments.GetDeploymentParams{
		DeploymentID:    c.deploymentID,
		ShowPlans:       ec.Bool(true),
		ShowPlanHistory: ec.Bool(includePlanHistory),
		Context:         ctx,
	}
	resp, err := c.ecAPI.V1API.Deployments.GetDeployment(params, c.ecAPI.AuthWriter)
	if err != nil {
		return nil, err
	}
	if len(resp.Payload.Resources.Elasticsearch) == 0 {
		return nil, errors.New("elasticsearch resource is not found")
	}
	return resp.Payload.Resources.Elasticsearch[0], nil
}

func (c *clientImpl) UpdateESHotContentTopologySize(ctx context.Context, updatedTopology *models.TopologySize) error {
	esResource, err := c.GetESResourceInfo(ctx, false)
	if err != nil {
		return err
	}
	for _, topology := range esResource.Info.PlanInfo.Current.Plan.ClusterTopology {
		if TopologyID(topology.ID) == TopologyIDHotContent {
			topology.Size = updatedTopology
		}
	}
	r := models.DeploymentUpdateRequest{
		PruneOrphans: func() *bool { b := false; return &b }(),
		Resources: &models.DeploymentUpdateResources{
			Elasticsearch: []*models.ElasticsearchPayload{
				{
					Plan:     esResource.Info.PlanInfo.Current.Plan,
					RefID:    esResource.RefID,
					Region:   esResource.Region,
					Settings: esResource.Info.Settings,
				},
			},
		},
	}
	_, err = deploymentapi.Update(deploymentapi.UpdateParams{
		DeploymentID: c.deploymentID,
		API:          c.ecAPI,
		Request:      &r,
	})
	if err != nil {
		return fmt.Errorf("update deployment: %w", err)
	}
	if err := WaitForPlanCompletion(c.ecAPI, c.deploymentID); err != nil {
		return fmt.Errorf("wait for plan completion: %w", err)
	}
	return nil
}

const (
	defaultPollPlanFrequency = 2 * time.Second
	defaultMaxPlanRetry      = 4
)

// WaitForPlanCompletion waits for a pending plan to finish.
func WaitForPlanCompletion(client *api.API, deploymentID string) error {
	return planutil.Wait(plan.TrackChangeParams{
		API:          client,
		DeploymentID: deploymentID,
		Config: plan.TrackFrequencyConfig{
			PollFrequency: defaultPollPlanFrequency,
			MaxRetries:    defaultMaxPlanRetry,
		},
	})
}
