package pipeline

import (
	"context"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/ingestion"
)

// FetchSource fetches a source and persists new RawItems (raw evidence is never
// overwritten). Returns the ids of newly created raw items. Mirrors fetchSource.ts.
// `now` is injected for determinism.
func FetchSource(ctx context.Context, d *db.DB, sourceID string, now time.Time) ([]string, error) {
	source, err := d.GetSourceForFetch(ctx, sourceID)
	if err != nil || source == nil || !source.Enabled {
		return nil, err
	}

	items, ferr := ingestion.FetchRSSSource(ctx, source.URL)
	if ferr != nil {
		if err := d.MarkSourceFetchFailure(ctx, sourceID, now); err != nil {
			return nil, err
		}
		return nil, nil
	}

	var newIDs []string
	for _, it := range items {
		if it.SourceGuid != nil {
			exists, err := d.RawItemExists(ctx, sourceID, *it.SourceGuid)
			if err != nil {
				return nil, err
			}
			if exists {
				continue
			}
		}
		id, err := d.CreateRawItem(ctx, db.NewRawItem{
			SourceID:    sourceID,
			SourceGuid:  it.SourceGuid,
			RawURL:      it.URL,
			RawTitle:    it.Title,
			RawContent:  it.Content,
			PublishedAt: it.PublishedAt,
		})
		if err != nil {
			return nil, err
		}
		if id != "" {
			newIDs = append(newIDs, id)
		}
	}

	if err := d.MarkSourceFetchSuccess(ctx, sourceID, now); err != nil {
		return nil, err
	}
	return newIDs, nil
}
