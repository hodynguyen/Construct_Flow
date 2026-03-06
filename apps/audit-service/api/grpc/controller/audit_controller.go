package controller

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hodynguyen/construct-flow/apps/audit-service/domain"
	auditv1 "github.com/hodynguyen/construct-flow/gen/go/proto/audit_service/v1"
)

type AuditController struct {
	auditv1.UnimplementedAuditServiceServer
	repo domain.AuditRepository
}

func NewAuditController(repo domain.AuditRepository) *AuditController {
	return &AuditController{repo: repo}
}

func (c *AuditController) QueryAuditLogs(ctx context.Context, req *auditv1.QueryAuditLogsRequest) (*auditv1.QueryAuditLogsResponse, error) {
	if req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}

	f := domain.QueryFilter{
		CompanyID:  req.CompanyId,
		Resource:   req.Resource,
		ResourceID: req.ResourceId,
		UserID:     req.UserId,
		Action:     req.Action,
		Page:       int(req.Page),
		PageSize:   int(req.PageSize),
	}
	if req.From != nil {
		t := req.From.AsTime()
		f.From = &t
	}
	if req.To != nil {
		t := req.To.AsTime()
		f.To = &t
	}

	logs, total, err := c.repo.Query(ctx, f)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "querying audit logs: %v", err)
	}

	resp := &auditv1.QueryAuditLogsResponse{Total: int32(total)}
	for _, l := range logs {
		resp.Logs = append(resp.Logs, &auditv1.AuditLog{
			Id:          l.ID,
			CompanyId:   l.CompanyID,
			UserId:      l.UserID,
			Action:      l.Action,
			Resource:    l.Resource,
			ResourceId:  l.ResourceID,
			BeforeState: l.BeforeState,
			AfterState:  l.AfterState,
			IpAddress:   l.IPAddress,
			OccurredAt:  timestamppb.New(l.OccurredAt),
		})
	}
	return resp, nil
}
