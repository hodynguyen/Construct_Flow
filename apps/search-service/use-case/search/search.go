package search

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hodynguyen/construct-flow/apps/search-service/service/elastic"
)

type Input struct {
	CompanyID string
	Query     string
	Types     []string // empty = all
	Page      int
	PageSize  int
}

type Hit struct {
	ID      string
	Type    string
	Title   string
	Snippet string
	Score   float32
}

type Output struct {
	Hits  []Hit
	Total int
}

type UseCase struct {
	es *elastic.Client
}

func New(es *elastic.Client) *UseCase {
	return &UseCase{es: es}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	if in.Page < 1 {
		in.Page = 1
	}
	if in.PageSize < 1 || in.PageSize > 50 {
		in.PageSize = 10
	}

	indices := resolveIndices(in.Types)
	allHits := []Hit{}
	totalCount := 0

	for _, idx := range indices {
		hits, total, err := uc.es.Search(ctx, idx, in.Query, in.Page, in.PageSize)
		if err != nil {
			// Best-effort: skip unavailable indices
			continue
		}
		totalCount += total
		for _, h := range hits {
			allHits = append(allHits, toHit(h, idx))
		}
	}

	return &Output{Hits: allHits, Total: totalCount}, nil
}

func resolveIndices(types []string) []string {
	all := []string{"tasks", "files", "reports"}
	if len(types) == 0 {
		return all
	}
	var indices []string
	for _, t := range types {
		switch strings.ToLower(t) {
		case "task":
			indices = append(indices, "tasks")
		case "file":
			indices = append(indices, "files")
		case "report":
			indices = append(indices, "reports")
		}
	}
	return indices
}

func toHit(h elastic.SearchHit, index string) Hit {
	var src map[string]interface{}
	_ = json.Unmarshal(h.Source, &src)

	title := ""
	for _, key := range []string{"title", "name", "type"} {
		if v, ok := src[key].(string); ok && v != "" {
			title = v
			break
		}
	}

	docType := strings.TrimSuffix(index, "s") // tasks → task

	return Hit{
		ID:    h.ID,
		Type:  docType,
		Title: title,
		Score: float32(h.Score),
	}
}
