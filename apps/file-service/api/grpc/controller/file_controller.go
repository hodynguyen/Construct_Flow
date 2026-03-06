package controller

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hodynguyen/construct-flow/apps/file-service/common"
	"github.com/hodynguyen/construct-flow/apps/file-service/domain"
	"github.com/hodynguyen/construct-flow/apps/file-service/entity/model"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/confirm_upload"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/get_download_url"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/migrate_storage"
	"github.com/hodynguyen/construct-flow/apps/file-service/use-case/presign_upload"
	filev1 "github.com/hodynguyen/construct-flow/gen/go/proto/file_service/v1"
)

type FileController struct {
	filev1.UnimplementedFileServiceServer
	repo           domain.FileRepository
	presignUC      *presign_upload.UseCase
	confirmUC      *confirm_upload.UseCase
	downloadUC     *get_download_url.UseCase
	migrateUC      *migrate_storage.UseCase
}

func NewFileController(
	repo domain.FileRepository,
	presignUC *presign_upload.UseCase,
	confirmUC *confirm_upload.UseCase,
	downloadUC *get_download_url.UseCase,
	migrateUC *migrate_storage.UseCase,
) *FileController {
	return &FileController{
		repo:       repo,
		presignUC:  presignUC,
		confirmUC:  confirmUC,
		downloadUC: downloadUC,
		migrateUC:  migrateUC,
	}
}

func (c *FileController) PresignUpload(ctx context.Context, req *filev1.PresignUploadRequest) (*filev1.PresignUploadResponse, error) {
	out, err := c.presignUC.Execute(ctx, presign_upload.Input{
		CompanyID:  req.CompanyId,
		UploadedBy: req.UploadedBy,
		Name:       req.Name,
		MimeType:   req.MimeType,
		SizeBytes:  req.SizeBytes,
		ProjectID:  req.ProjectId,
		TaskID:     req.TaskId,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &filev1.PresignUploadResponse{
		FileId:    out.FileID,
		UploadUrl: out.UploadURL,
		S3Key:     out.S3Key,
	}, nil
}

func (c *FileController) ConfirmUpload(ctx context.Context, req *filev1.ConfirmUploadRequest) (*filev1.FileResponse, error) {
	file, err := c.confirmUC.Execute(ctx, req.CompanyId, req.FileId, req.SizeBytes)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &filev1.FileResponse{File: toProtoFile(file)}, nil
}

func (c *FileController) GetDownloadURL(ctx context.Context, req *filev1.GetDownloadURLRequest) (*filev1.GetDownloadURLResponse, error) {
	out, err := c.downloadUC.Execute(ctx, req.CompanyId, req.FileId)
	if errors.Is(err, common.ErrGlacierRestore) {
		return &filev1.GetDownloadURLResponse{
			StorageTier:     "glacier",
			RequiresRestore: true,
		}, nil
	}
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &filev1.GetDownloadURLResponse{
		DownloadUrl:     out.DownloadURL,
		StorageTier:     out.StorageTier,
		RequiresRestore: false,
	}, nil
}

func (c *FileController) ListFiles(ctx context.Context, req *filev1.ListFilesRequest) (*filev1.ListFilesResponse, error) {
	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize < 1 {
		pageSize = 20
	}
	files, total, err := c.repo.ListByResource(ctx, req.CompanyId, req.ProjectId, req.TaskId, page, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}
	var protoFiles []*filev1.File
	for i := range files {
		protoFiles = append(protoFiles, toProtoFile(&files[i]))
	}
	return &filev1.ListFilesResponse{
		Files:    protoFiles,
		Total:    int32(total),
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (c *FileController) DeleteFile(ctx context.Context, req *filev1.DeleteFileRequest) (*emptypb.Empty, error) {
	if err := c.repo.SoftDelete(ctx, req.CompanyId, req.FileId); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *FileController) MigrateStorage(ctx context.Context, req *filev1.MigrateStorageRequest) (*filev1.MigrateStorageResponse, error) {
	count, err := c.migrateUC.Execute(ctx, req.CompanyId, req.TargetTier)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &filev1.MigrateStorageResponse{MigratedCount: int32(count)}, nil
}

func toProtoFile(f *model.File) *filev1.File {
	proto := &filev1.File{
		Id:          f.ID,
		CompanyId:   f.CompanyID,
		UploadedBy:  f.UploadedBy,
		Name:        f.Name,
		MimeType:    f.MimeType,
		SizeBytes:   f.SizeBytes,
		StorageTier: f.StorageTier,
		Status:      f.Status,
		CreatedAt:   timestamppb.New(f.CreatedAt),
	}
	if f.ProjectID != nil {
		proto.ProjectId = *f.ProjectID
	}
	if f.TaskID != nil {
		proto.TaskId = *f.TaskID
	}
	return proto
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, common.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, common.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, common.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
