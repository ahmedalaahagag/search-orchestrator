package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ahmedalaahagag/search-orchestrator/internal/infra/observability"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/opensearch"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/qus"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/query"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/config"
	"github.com/sirupsen/logrus"
)

type Orchestrator struct {
	logger  *logrus.Logger
	metrics *observability.Metrics
	qus     qus.QUSClient
	os      opensearch.OpenSearchClient
	planner *Planner
	cfg     config.SearchConfig
}

func New(
	logger *logrus.Logger,
	metrics *observability.Metrics,
	qusClient qus.QUSClient,
	osClient opensearch.OpenSearchClient,
	planner *Planner,
	cfg config.SearchConfig,
) *Orchestrator {
	return &Orchestrator{
		logger:  logger,
		metrics: metrics,
		qus:     qusClient,
		os:      osClient,
		planner: planner,
		cfg:     cfg,
	}
}

func (o *Orchestrator) Search(ctx context.Context, req model.SearchRequest) (*model.SearchResponse, error) {
	// Step 1: Call QUS with graceful fallback.
	qusResp, warnings := o.callQUS(ctx, req)

	// Step 2: Build search plan.
	plan := o.planner.BuildPlan(req, qusResp)

	// Step 3: Execute stages with threshold fallback.
	osResp, stageName, err := o.executeStages(ctx, plan)
	if err != nil {
		return nil, err
	}

	if o.metrics != nil {
		o.metrics.StageApplied.WithLabelValues(stageName).Inc()
	}

	// Step 4: Map results.
	items := mapHits(osResp)
	facets := parseFacets(osResp, o.cfg.Facets)
	cursor := buildCursor(osResp)
	hasNext := len(osResp.Hits.Hits) == plan.PageSize

	return &model.SearchResponse{
		Items:  items,
		Facets: facets,
		Page: model.PageInfo{
			Size:        plan.PageSize,
			HasNextPage: hasNext,
			Cursor:      cursor,
		},
		Meta: model.SearchMeta{
			TotalHits: osResp.Hits.Total.Value,
			Stage:     stageName,
			Warnings:  warnings,
		},
	}, nil
}

func (o *Orchestrator) callQUS(ctx context.Context, req model.SearchRequest) (*model.QUSAnalyzeResponse, []string) {
	start := time.Now()

	qusResp, err := o.qus.Analyze(ctx, model.QUSAnalyzeRequest{
		Query:  req.Query,
		Locale: req.Locale,
		Market: req.Market,
	})

	if o.metrics != nil {
		o.metrics.QUSDuration.Observe(time.Since(start).Seconds())
	}

	if err != nil {
		o.logger.WithError(err).Warn("QUS call failed, using degraded mode")
		if o.metrics != nil {
			o.metrics.QUSFailures.Inc()
		}
		return nil, []string{"QUS unavailable, using raw query"}
	}

	var warnings []string
	warnings = append(warnings, qusResp.Warnings...)
	return qusResp, warnings
}

func (o *Orchestrator) executeStages(ctx context.Context, plan model.SearchPlan) (*opensearch.SearchResponse, string, error) {
	var lastResp *opensearch.SearchResponse
	var lastStage string

	for _, stage := range plan.Stages {
		start := time.Now()

		stageQuery := query.BuildStageQuery(plan.Tokens, stage)
		body, err := query.BuildFullRequest(stageQuery, plan)
		if err != nil {
			return nil, "", err
		}

		raw, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}

		index := resolveIndex(o.cfg.Index, plan.Market)
		resp, err := o.os.Search(ctx, index, raw)

		if o.metrics != nil {
			o.metrics.StageDuration.WithLabelValues(stage.Name).Observe(time.Since(start).Seconds())
		}

		if err != nil {
			o.logger.WithError(err).WithField("stage", stage.Name).Error("stage execution failed")
			return nil, "", err
		}

		lastResp = resp
		lastStage = stage.Name

		if resp.Hits.Total.Value >= stage.MinimumHits {
			return resp, stage.Name, nil
		}

		o.logger.WithFields(logrus.Fields{
			"stage":    stage.Name,
			"hits":     resp.Hits.Total.Value,
			"required": stage.MinimumHits,
		}).Debug("stage below threshold, trying next")
	}

	if lastResp == nil {
		return &opensearch.SearchResponse{}, lastStage, nil
	}

	return lastResp, lastStage, nil
}

func resolveIndex(template, market string) string {
	return strings.ReplaceAll(template, "{market}", strings.ToLower(market))
}

type sourceDoc struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Headline    string   `json:"headline,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Allergens   []string `json:"allergens,omitempty"`
	Ingredients []string `json:"ingredients,omitempty"`
	SoldOut     bool     `json:"sold_out"`
	Active      bool     `json:"active"`
}

func mapHits(resp *opensearch.SearchResponse) []model.SearchItem {
	items := make([]model.SearchItem, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		var doc sourceDoc
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			continue
		}

		id := doc.ID
		if id == "" {
			id = hit.ID
		}

		items = append(items, model.SearchItem{
			ID:          id,
			Title:       doc.Title,
			Score:       hit.Score,
			Description: doc.Description,
			Headline:    doc.Headline,
			Slug:        doc.Slug,
			ImageURL:    doc.ImageURL,
			Categories:  doc.Categories,
			Tags:        doc.Tags,
			Allergens:   doc.Allergens,
			Ingredients: doc.Ingredients,
		})
	}
	return items
}

func parseFacets(resp *opensearch.SearchResponse, facetCfgs []config.FacetConfig) []model.FacetResult {
	if resp.Aggregations == nil {
		return nil
	}

	var results []model.FacetResult
	for _, fc := range facetCfgs {
		aggKey := "agg_" + fc.Field
		raw, ok := resp.Aggregations[aggKey]
		if !ok {
			continue
		}

		// Parse the outer filter aggregation wrapper.
		var wrapper struct {
			DocCount int                        `json:"doc_count"`
			Inner    map[string]json.RawMessage `json:"-"`
		}
		// We need to unmarshal the whole thing as a map to find the nested agg.
		var fullAgg map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fullAgg); err != nil {
			continue
		}

		innerRaw, ok := fullAgg[fc.Field]
		if !ok {
			// Try without wrapper (direct agg).
			_ = json.Unmarshal(raw, &wrapper)
			continue
		}

		var aggResult opensearch.AggResult
		if err := json.Unmarshal(innerRaw, &aggResult); err != nil {
			continue
		}

		buckets := make([]model.FacetBucket, 0, len(aggResult.Buckets))
		for _, b := range aggResult.Buckets {
			buckets = append(buckets, model.FacetBucket{
				Key:   b.Key,
				Count: b.DocCount,
			})
		}

		results = append(results, model.FacetResult{
			Field:   fc.Field,
			Buckets: buckets,
		})
	}

	return results
}

func buildCursor(resp *opensearch.SearchResponse) string {
	if len(resp.Hits.Hits) == 0 {
		return ""
	}
	lastHit := resp.Hits.Hits[len(resp.Hits.Hits)-1]
	return query.EncodeCursor(lastHit.Sort)
}
