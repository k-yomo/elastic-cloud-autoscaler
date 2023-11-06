package elasticcloud

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/mock"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	planmock "github.com/elastic/cloud-sdk-go/pkg/plan/mock"
	"github.com/elastic/cloud-sdk-go/pkg/util/ec"
)

const mockDeploymentID = "11111111111111111111111111111111"

var defaultMockDeploymentGetResponse = &models.DeploymentGetResponse{
	Healthy: ec.Bool(true),
	ID:      ec.String(mock.ValidClusterID),
	Resources: &models.DeploymentResources{
		Elasticsearch: []*models.ElasticsearchResourceInfo{
			{
				Info: &models.ElasticsearchClusterInfo{
					PlanInfo: &models.ElasticsearchClusterPlansInfo{
						Current: &models.ElasticsearchClusterPlanInfo{
							Plan: &models.ElasticsearchClusterPlan{
								ClusterTopology: []*models.ElasticsearchClusterTopologyElement{
									{
										ID:   string(TopologyIDHotContent),
										Size: NewTopologySize(256),
									},
									{
										ID:   string(TopologyIDMaster),
										Size: NewTopologySize(64),
									},
								},
								Transient: &models.TransientElasticsearchPlanConfiguration{
									PlanConfiguration: &models.ElasticsearchPlanControlConfiguration{
										MoveOnly: ec.Bool(true),
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

func Test_clientImpl_UpdateESHotContentTopologySize(t *testing.T) {
	t.Parallel()

	defaultPollPlanFrequency = 1 * time.Millisecond
	defaultMaxPlanRetry = 1

	tests := []struct {
		name            string
		ecAPI           *api.API
		deploymentID    string
		updatedTopology *models.TopologySize
		wantErr         bool
	}{
		{
			name:            "returns null when topology is updated successfully",
			deploymentID:    mockDeploymentID,
			updatedTopology: NewTopologySize(512),
			ecAPI: api.NewMock(
				mock.New200Response(mock.NewStructBody(defaultMockDeploymentGetResponse)),
				mock.New200ResponseAssertion(
					&mock.RequestAssertion{
						Host:   api.DefaultMockHost,
						Header: api.DefaultWriteMockHeaders,
						Method: "PUT",
						Path:   fmt.Sprintf("/api/v1/deployments/%s", mockDeploymentID),
						Query: url.Values{
							"hide_pruned_orphans": []string{"false"},
							"skip_snapshot":       []string{"false"},
						},
						Body: mock.NewStructBody(
							models.DeploymentUpdateRequest{
								PruneOrphans: ec.Bool(false),
								Resources: &models.DeploymentUpdateResources{
									Elasticsearch: []*models.ElasticsearchPayload{
										{
											Plan: &models.ElasticsearchClusterPlan{
												ClusterTopology: []*models.ElasticsearchClusterTopologyElement{
													{
														ID:   string(TopologyIDHotContent),
														Size: NewTopologySize(512),
													},
													{
														ID:   string(TopologyIDMaster),
														Size: NewTopologySize(64),
													},
												},
												// plan configuration is set to nil
												Transient: &models.TransientElasticsearchPlanConfiguration{
													Strategy: &models.PlanStrategy{
														Autodetect: struct{}{},
													},
												},
											},
										},
									},
								},
							},
						),
					},
					mock.NewStringBody(`{}`), // dummy
				),
				mock.New200Response(mock.NewStructBody(
					planmock.Generate(planmock.GenerateConfig{
						ID:            "cbb4bc6c09684c86aa5de54c05ea1d38",
						Elasticsearch: []planmock.GeneratedResourceConfig{},
					})),
				),
				mock.New200Response(mock.NewStructBody(
					planmock.Generate(planmock.GenerateConfig{
						ID: "cbb4bc6c09684c86aa5de54c05ea1d38",
						Elasticsearch: []planmock.GeneratedResourceConfig{
							{
								ID:         "cde7b6b605424a54ce9d56316eab13a1",
								PendingLog: nil,
								CurrentLog: planmock.NewPlanStepLog(
									planmock.NewPlanStep("plan-completed", "success"),
								),
							},
						},
					})),
				),
				mock.New200Response(mock.NewStructBody(defaultMockDeploymentGetResponse)),
			),
			wantErr: false,
		},
		{
			name:            "returns error when topology update plan completed with error",
			deploymentID:    mockDeploymentID,
			updatedTopology: NewTopologySize(512),
			ecAPI: api.NewMock(
				mock.New200Response(mock.NewStructBody(defaultMockDeploymentGetResponse)),
				mock.New200Response(mock.NewStringBody(`{}`)), // updated
				mock.New200Response(mock.NewStructBody(
					planmock.Generate(planmock.GenerateConfig{
						ID: mockDeploymentID,
						Elasticsearch: []planmock.GeneratedResourceConfig{
							{
								ID:         "cde7b6b605424a54ce9d56316eab13a1",
								PendingLog: nil,
								CurrentLog: planmock.NewPlanStepLog(
									planmock.NewPlanStep("step-1", "success"),
									planmock.NewPlanStep("plan-completed", "error"),
								),
							},
						},
					})),
				),
				mock.New200Response(mock.NewStructBody(
					planmock.Generate(planmock.GenerateConfig{
						ID: mockDeploymentID,
						Elasticsearch: []planmock.GeneratedResourceConfig{
							{
								ID:         "cde7b6b605424a54ce9d56316eab13a1",
								PendingLog: nil,
								CurrentLog: planmock.NewPlanStepLog(
									planmock.NewPlanStep("step-1", "success"),
									planmock.NewPlanStepWithDetailsAndError("plan-completed", []*models.ClusterPlanStepLogMessageInfo{
										{Message: ec.String("failure")},
									}),
								),
							},
						},
					})),
				),
			),
			wantErr: true,
		},
		{
			name:            "returns error when updated plan has error",
			deploymentID:    mockDeploymentID,
			updatedTopology: NewTopologySize(512),
			ecAPI: api.NewMock(
				mock.New200Response(mock.NewStructBody(defaultMockDeploymentGetResponse)),
				mock.New200Response(mock.NewStringBody(`{}`)), // updated
				mock.New200Response(mock.NewStructBody(
					planmock.Generate(planmock.GenerateConfig{
						ID: mockDeploymentID,
						Elasticsearch: []planmock.GeneratedResourceConfig{
							{
								ID:         "cde7b6b605424a54ce9d56316eab13a1",
								PendingLog: nil,
								CurrentLog: planmock.NewPlanStepLog(
									planmock.NewPlanStep("step-1", "success"),
									planmock.NewPlanStep("plan-completed", "success"),
								),
							},
						},
					})),
				),
				mock.New200Response(mock.NewStructBody(
					planmock.Generate(planmock.GenerateConfig{
						ID: mockDeploymentID,
						Elasticsearch: []planmock.GeneratedResourceConfig{
							{
								ID:         "cde7b6b605424a54ce9d56316eab13a1",
								PendingLog: nil,
								CurrentLog: planmock.NewPlanStepLog(
									planmock.NewPlanStep("step-1", "success"),
									planmock.NewPlanStep("plan-completed", "success"),
								),
							},
						},
					})),
				),
				mock.New200Response(mock.NewStructBody(&models.DeploymentGetResponse{
					Healthy: ec.Bool(false),
					ID:      ec.String(mock.ValidClusterID),
					Resources: &models.DeploymentResources{
						Elasticsearch: []*models.ElasticsearchResourceInfo{
							{
								Info: &models.ElasticsearchClusterInfo{
									PlanInfo: &models.ElasticsearchClusterPlansInfo{
										Current: &models.ElasticsearchClusterPlanInfo{
											Error: &models.ClusterPlanAttemptError{
												FailureType: "InfrastructureFailure:NotEnoughCapacity",
												Message:     ec.String("Plan change failed: Not enough capacity to allocate instance(s)"),
											},
										},
									},
								},
							},
						},
					},
				},
				)),
			),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := &clientImpl{
				ecAPI:        tt.ecAPI,
				deploymentID: tt.deploymentID,
			}
			err := c.UpdateESHotContentTopologySize(context.Background(), tt.updatedTopology)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateESHotContentTopologySize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
