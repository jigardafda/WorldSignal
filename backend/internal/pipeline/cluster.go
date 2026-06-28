package pipeline

import (
	"context"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/textutil"
)

const (
	similarityThreshold = 0.5
	windowHours         = 72
)

// ClusterResult mirrors the TS ClusterResult.
type ClusterResult struct {
	SignalID string
	IsNew    bool
}

// ClusterArticle attaches an article to an existing Signal or creates a new one,
// using token-set Jaccard similarity over a recent window. Mirrors cluster.ts.
// `now` is injected for determinism in tests.
func ClusterArticle(ctx context.Context, d *db.DB, articleID string, now time.Time) (*ClusterResult, error) {
	article, err := d.GetClusterArticle(ctx, articleID)
	if err != nil || article == nil {
		return nil, err
	}

	existing, err := d.ExistingSignalForArticle(ctx, articleID)
	if err != nil {
		return nil, err
	}
	if existing != "" {
		return &ClusterResult{SignalID: existing, IsNew: false}, nil
	}

	since := now.Add(-windowHours * time.Hour)
	candidates, err := d.RecentSignalCandidates(ctx, since)
	if err != nil {
		return nil, err
	}

	bestID := ""
	bestScore := 0.0
	first := true
	for _, c := range candidates {
		score := textutil.Jaccard(article.TokenSet, c.TokenSet)
		if first || score > bestScore {
			bestID = c.ID
			bestScore = score
			first = false
		}
	}

	if bestID != "" && bestScore >= similarityThreshold {
		if err := d.AttachArticleToSignal(ctx, bestID, articleID, bestScore, now); err != nil {
			return nil, err
		}
		return &ClusterResult{SignalID: bestID, IsNew: false}, nil
	}

	id, err := d.CreateSignalFromArticle(ctx, article, now)
	if err != nil {
		return nil, err
	}
	return &ClusterResult{SignalID: id, IsNew: true}, nil
}
