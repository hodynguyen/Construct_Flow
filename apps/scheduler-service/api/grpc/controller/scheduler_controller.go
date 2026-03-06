package controller

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hodynguyen/construct-flow/apps/scheduler-service/bootstrap"
	schedulerv1 "github.com/hodynguyen/construct-flow/gen/go/proto/scheduler_service/v1"
)

type SchedulerController struct {
	schedulerv1.UnimplementedSchedulerServiceServer
	scheduler *bootstrap.Scheduler
}

func NewSchedulerController(scheduler *bootstrap.Scheduler) *SchedulerController {
	return &SchedulerController{scheduler: scheduler}
}

func (c *SchedulerController) ListJobs(_ context.Context, _ *schedulerv1.ListJobsRequest) (*schedulerv1.ListJobsResponse, error) {
	metas := c.scheduler.Jobs()
	resp := &schedulerv1.ListJobsResponse{}
	for _, m := range metas {
		resp.Jobs = append(resp.Jobs, &schedulerv1.Job{
			Name:     m.Name,
			Schedule: m.Schedule,
			Status:   "active",
		})
	}
	return resp, nil
}

func (c *SchedulerController) TriggerJob(_ context.Context, req *schedulerv1.TriggerJobRequest) (*schedulerv1.TriggerJobResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	// Manual trigger is not supported in this demo implementation
	return nil, status.Error(codes.Unimplemented, "manual trigger not implemented")
}
