package jobs

import (
	"context"
	"net/http"
	"time"

	"github.com/worldsignal/backend/internal/crawl"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/jsonx"
	"github.com/worldsignal/backend/internal/llm"
	"github.com/worldsignal/backend/internal/logging"
	"github.com/worldsignal/backend/internal/pipeline"
	"github.com/worldsignal/backend/internal/stream"
)

// Queue names, mirroring queues.ts.
const (
	QFetchSource    = "source.fetch"
	QProcessArticle = "article.process"
	QEnrichSignal   = "signal.enrich"
	QMatchSignal    = "signal.match"
	QSendDelivery   = "delivery.send"
)

const deliveryRetryLimit = 5

// Workers wires the pipeline stages onto the queue and exposes enqueue helpers.
type Workers struct {
	Q       *Queue
	DB      *db.DB
	Gateway llm.Gateway
	Crawler pipeline.PageCrawler
	Client  *http.Client
	Secret  string
	// Hub, when set, is notified per subscription as deliveries are created so
	// streaming (SSE/WebSocket) clients wake immediately. Optional (nil = no-op).
	Hub *stream.Hub
	log *logging.Logger
	// Source failure handling: after FailureThreshold consecutive fetch
	// failures, a source is placed in cooldown for Cooldown.
	FailureThreshold int
	Cooldown         time.Duration
}

// NewWorkers builds the worker set with sensible cooldown defaults (overridable).
func NewWorkers(q *Queue, d *db.DB, gw llm.Gateway, secret string) *Workers {
	return &Workers{
		Q: q, DB: d, Gateway: gw, Crawler: crawl.New(),
		Client: &http.Client{Timeout: 10 * time.Second},
		Secret: secret, log: logging.New("workers"),
		FailureThreshold: 5, Cooldown: 3 * time.Hour,
	}
}

// EnqueueFetchSource enqueues a source fetch (deduped per source via singleton).
func (w *Workers) EnqueueFetchSource(sourceID string) error {
	return w.Q.Send(context.Background(), QFetchSource, map[string]string{"sourceId": sourceID},
		SendOptions{SingletonKey: "fetch:" + sourceID})
}

func (w *Workers) enqueueProcessArticle(ctx context.Context, rawItemID string) error {
	return w.Q.Send(ctx, QProcessArticle, map[string]string{"rawItemId": rawItemID}, SendOptions{})
}
func (w *Workers) enqueueEnrichSignal(ctx context.Context, signalID string) error {
	return w.Q.Send(ctx, QEnrichSignal, map[string]string{"signalId": signalID}, SendOptions{})
}
func (w *Workers) enqueueMatchSignal(ctx context.Context, signalID string) error {
	return w.Q.Send(ctx, QMatchSignal, map[string]string{"signalId": signalID}, SendOptions{})
}
func (w *Workers) enqueueSendDelivery(ctx context.Context, deliveryID string) error {
	return w.Q.Send(ctx, QSendDelivery, map[string]string{"deliveryId": deliveryID},
		SendOptions{RetryLimit: deliveryRetryLimit, RetryDelay: 5, RetryBackoff: true})
}

// notifyStream wakes streaming clients for a subscription (no-op if no hub).
func (w *Workers) notifyStream(subID string) {
	if w.Hub != nil {
		w.Hub.Notify(subID)
	}
}

// EnqueueSendDelivery enqueues a delivery send (used by the API retry action).
func (w *Workers) EnqueueSendDelivery(deliveryID string) error {
	return w.enqueueSendDelivery(context.Background(), deliveryID)
}

// Register attaches all pipeline handlers to the queue.
func (w *Workers) Register() {
	w.Q.RegisterWorker(QFetchSource, func(ctx context.Context, data []byte, _ bool) error {
		var j struct {
			SourceID string `json:"sourceId"`
		}
		if err := jsonx.Unmarshal(data, &j); err != nil {
			return err
		}
		rawIDs, err := pipeline.FetchSource(ctx, w.DB, j.SourceID, time.Now(), w.FailureThreshold, w.Cooldown)
		if err != nil {
			return err
		}
		for _, id := range rawIDs {
			if err := w.enqueueProcessArticle(ctx, id); err != nil {
				return err
			}
		}
		return nil
	})

	w.Q.RegisterWorker(QProcessArticle, func(ctx context.Context, data []byte, _ bool) error {
		var j struct {
			RawItemID string `json:"rawItemId"`
		}
		if err := jsonx.Unmarshal(data, &j); err != nil {
			return err
		}
		articleID, err := pipeline.NormalizeRawItem(ctx, w.DB, j.RawItemID)
		if err != nil || articleID == "" {
			return err
		}
		cluster, err := pipeline.ClusterArticle(ctx, w.DB, articleID, time.Now())
		if err != nil || cluster == nil {
			return err
		}
		return w.enqueueEnrichSignal(ctx, cluster.SignalID)
	})

	w.Q.RegisterWorker(QEnrichSignal, func(ctx context.Context, data []byte, _ bool) error {
		var j struct {
			SignalID string `json:"signalId"`
		}
		if err := jsonx.Unmarshal(data, &j); err != nil {
			return err
		}
		if err := pipeline.EnrichSignal(ctx, w.DB, w.Gateway, w.Crawler, j.SignalID, time.Now()); err != nil {
			return err
		}
		return w.enqueueMatchSignal(ctx, j.SignalID)
	})

	w.Q.RegisterWorker(QMatchSignal, func(ctx context.Context, data []byte, _ bool) error {
		var j struct {
			SignalID string `json:"signalId"`
		}
		if err := jsonx.Unmarshal(data, &j); err != nil {
			return err
		}
		ids, err := pipeline.MatchSubscriptions(ctx, w.DB, j.SignalID, time.Now(), w.notifyStream)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := w.enqueueSendDelivery(ctx, id); err != nil {
				return err
			}
		}
		return nil
	})

	w.Q.RegisterWorker(QSendDelivery, func(ctx context.Context, data []byte, isFinal bool) error {
		var j struct {
			DeliveryID string `json:"deliveryId"`
		}
		if err := jsonx.Unmarshal(data, &j); err != nil {
			return err
		}
		return pipeline.SendDelivery(ctx, w.DB, w.Client, w.Secret, j.DeliveryID, isFinal, time.Now())
	})

	w.log.Info("workers registered")
}
