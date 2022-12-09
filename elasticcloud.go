package main

import (
	"errors"
	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi/deputil"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/plan"
	"github.com/elastic/cloud-sdk-go/pkg/plan/planutil"
	"time"
)

type Client struct {
	ecAPI        *api.API
	deploymentID string
	cloudID      string
}

func NewClient(ecAPI *api.API, deploymentID string) (*Client, error) {
	resp, err := deploymentapi.Get(deploymentapi.GetParams{
		API:          ecAPI,
		DeploymentID: deploymentID,
		QueryParams: deputil.QueryParams{
			ShowPlans: true,
		},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Resources.Elasticsearch) == 0 {
		return nil, errors.New("elasticsearch resource is not found")
	}
	return &Client{
		ecAPI:        ecAPI,
		deploymentID: deploymentID,
		cloudID:      resp.Resources.Elasticsearch[0].Info.Metadata.CloudID,
	}, nil
}

func (c *Client) UpdateElasticsearchTopologySize(topologyIDSizeMap map[string]*models.TopologySize) error {
	resp, err := deploymentapi.Get(deploymentapi.GetParams{
		API:          c.ecAPI,
		DeploymentID: c.deploymentID,
		QueryParams: deputil.QueryParams{
			ShowPlans: true,
		},
	})
	if err != nil {
		return err
	}
	if len(resp.Resources.Elasticsearch) == 0 {
		return err
	}
	esResource := resp.Resources.Elasticsearch[0]
	for _, topology := range esResource.Info.PlanInfo.Current.Plan.ClusterTopology {
		if size, ok := topologyIDSizeMap[topology.ID]; ok {
			topology.Size = size
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
		return err
	}
	if err := WaitForPlanCompletion(c.ecAPI, c.deploymentID); err != nil {
		return err
	}
	return nil
}

const (
	defaultPollPlanFrequency = 2 * time.Second
	defaultMaxPlanRetry      = 4
)

// WaitForPlanCompletion waits for a pending plan to finish.
func WaitForPlanCompletion(client *api.API, id string) error {
	return planutil.Wait(plan.TrackChangeParams{
		API: client, DeploymentID: id,
		Config: plan.TrackFrequencyConfig{
			PollFrequency: defaultPollPlanFrequency,
			MaxRetries:    defaultMaxPlanRetry,
		},
	})
}
