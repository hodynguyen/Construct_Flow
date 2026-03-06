package controller

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hodynguyen/construct-flow/apps/search-service/use-case/search"
	searchv1 "github.com/hodynguyen/construct-flow/gen/go/proto/search_service/v1"
)

type SearchController struct {
	searchv1.UnimplementedSearchServiceServer
	searchUC *search.UseCase
}

func NewSearchController(searchUC *search.UseCase) *SearchController {
	return &SearchController{searchUC: searchUC}
}

func (c *SearchController) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	if req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}
	if req.Q == "" {
		return nil, status.Error(codes.InvalidArgument, "q is required")
	}

	out, err := c.searchUC.Execute(ctx, search.Input{
		CompanyID: req.CompanyId,
		Query:     req.Q,
		Types:     req.Types,
		Page:      int(req.Page),
		PageSize:  int(req.PageSize),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "searching: %v", err)
	}

	resp := &searchv1.SearchResponse{Total: int32(out.Total)}
	for _, h := range out.Hits {
		resp.Hits = append(resp.Hits, &searchv1.SearchHit{
			Id:      h.ID,
			Type:    h.Type,
			Title:   h.Title,
			Snippet: h.Snippet,
			Score:   h.Score,
		})
	}
	return resp, nil
}
