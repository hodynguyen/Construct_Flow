package controller

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hodynguyen/construct-flow/apps/report-service/common"
	"github.com/hodynguyen/construct-flow/apps/report-service/domain"
	"github.com/hodynguyen/construct-flow/apps/report-service/use-case/get_report_status"
	"github.com/hodynguyen/construct-flow/apps/report-service/use-case/request_report"
	reportv1 "github.com/hodynguyen/construct-flow/gen/go/proto/report_service/v1"
)

type ReportController struct {
	reportv1.UnimplementedReportServiceServer
	requestReportUC  *request_report.UseCase
	getStatusUC      *get_report_status.UseCase
	repo             domain.ReportRepository
}

func NewReportController(
	requestReportUC *request_report.UseCase,
	getStatusUC *get_report_status.UseCase,
	repo domain.ReportRepository,
) *ReportController {
	return &ReportController{
		requestReportUC: requestReportUC,
		getStatusUC:     getStatusUC,
		repo:            repo,
	}
}

func (c *ReportController) RequestReport(ctx context.Context, req *reportv1.RequestReportRequest) (*reportv1.RequestReportResponse, error) {
	if req.CompanyId == "" || req.RequestedBy == "" || req.Type == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id, requested_by, and type are required")
	}

	params := buildParams(req)
	out, err := c.requestReportUC.Execute(ctx, request_report.Input{
		CompanyID:   req.CompanyId,
		RequestedBy: req.RequestedBy,
		JobType:     req.Type,
		Params:      params,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "requesting report: %v", err)
	}

	return &reportv1.RequestReportResponse{JobId: out.JobID}, nil
}

func (c *ReportController) GetReportStatus(ctx context.Context, req *reportv1.GetReportStatusRequest) (*reportv1.ReportJob, error) {
	if req.CompanyId == "" || req.JobId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id and job_id are required")
	}

	job, err := c.getStatusUC.Execute(ctx, req.CompanyId, req.JobId)
	if err == common.ErrNotFound {
		return nil, status.Error(codes.NotFound, "report job not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting report status: %v", err)
	}

	pb := &reportv1.ReportJob{
		Id:          job.ID,
		CompanyId:   job.CompanyID,
		RequestedBy: job.RequestedBy,
		Type:        job.Type,
		Params:      job.Params,
		Status:      job.Status,
		DownloadUrl: job.DownloadURL,
		CreatedAt:   timestamppb.New(job.CreatedAt),
	}
	if job.ErrorMsg != nil {
		pb.ErrorMsg = *job.ErrorMsg
	}
	if job.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*job.CompletedAt)
	}
	return pb, nil
}

func (c *ReportController) ListReports(ctx context.Context, req *reportv1.ListReportsRequest) (*reportv1.ListReportsResponse, error) {
	if req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}

	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	jobs, total, err := c.repo.List(ctx, req.CompanyId, req.RequestedBy, req.Status, page, pageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing reports: %v", err)
	}

	resp := &reportv1.ListReportsResponse{Total: int32(total)}
	for _, j := range jobs {
		pb := &reportv1.ReportJob{
			Id:          j.ID,
			CompanyId:   j.CompanyID,
			RequestedBy: j.RequestedBy,
			Type:        j.Type,
			Status:      j.Status,
			DownloadUrl: j.DownloadURL,
			CreatedAt:   timestamppb.New(j.CreatedAt),
		}
		if j.ErrorMsg != nil {
			pb.ErrorMsg = *j.ErrorMsg
		}
		if j.CompletedAt != nil {
			pb.CompletedAt = timestamppb.New(*j.CompletedAt)
		}
		resp.Reports = append(resp.Reports, pb)
	}
	return resp, nil
}

func buildParams(req *reportv1.RequestReportRequest) string {
	// Encode optional fields as a simple JSON string for storage.
	if req.ProjectId == "" && req.Week == "" {
		return "{}"
	}
	return `{"project_id":"` + req.ProjectId + `","week":"` + req.Week + `"}`
}
