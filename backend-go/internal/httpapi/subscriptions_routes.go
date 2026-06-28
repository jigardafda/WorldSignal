package httpapi

import (
	"net/http"
	"strconv"

	"github.com/worldsignal/backend/internal/db"
)

func (s *Server) registerSubscriptionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/subscriptions", s.listSubscriptions)
	mux.HandleFunc("GET /v1/deliveries", s.listDeliveries)
}

type restSubscriber struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	CreatedAt db.PrismaTime `json:"createdAt"`
}

type restCount struct {
	Deliveries int `json:"deliveries"`
}

// restSubscriptionFull mirrors GET /v1/subscriptions rows: scalar fields then the
// subscriber relation then the _count aggregate.
type restSubscriptionFull struct {
	ID           string         `json:"id"`
	SubscriberID string         `json:"subscriberId"`
	Name         string         `json:"name"`
	Channel      string         `json:"channel"`
	Filter       db.RawJSON     `json:"filter"`
	Config       db.RawJSON     `json:"config"`
	Enabled      bool           `json:"enabled"`
	CreatedAt    db.PrismaTime  `json:"createdAt"`
	Subscriber   restSubscriber `json:"subscriber"`
	Count        restCount      `json:"_count"`
}

// restSubscriptionScalar mirrors a Subscription without includes.
type restSubscriptionScalar struct {
	ID           string        `json:"id"`
	SubscriberID string        `json:"subscriberId"`
	Name         string        `json:"name"`
	Channel      string        `json:"channel"`
	Filter       db.RawJSON    `json:"filter"`
	Config       db.RawJSON    `json:"config"`
	Enabled      bool          `json:"enabled"`
	CreatedAt    db.PrismaTime `json:"createdAt"`
}

func (s *Server) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := s.DB.ListSubscriptions(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]restSubscriptionFull, len(subs))
	for i, sub := range subs {
		out[i] = restSubscriptionFull{
			ID: sub.ID, SubscriberID: sub.SubscriberID, Name: sub.Name, Channel: sub.Channel,
			Filter: sub.Filter, Config: sub.Config, Enabled: sub.Enabled, CreatedAt: db.NewTime(sub.CreatedAt),
			Subscriber: restSubscriber{
				ID: sub.Subscriber.ID, Name: sub.Subscriber.Name,
				Status: sub.Subscriber.Status, CreatedAt: db.NewTime(sub.Subscriber.CreatedAt),
			},
			Count: restCount{Deliveries: sub.DeliveryCount},
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Data []restSubscriptionFull `json:"data"`
	}{out})
}

type restSignalTitle struct {
	Title string `json:"title"`
}

// restDelivery mirrors GET /v1/deliveries rows: scalars, subscription, signal{title}.
type restDelivery struct {
	ID             string                 `json:"id"`
	SubscriptionID string                 `json:"subscriptionId"`
	SignalID       string                 `json:"signalId"`
	Channel        string                 `json:"channel"`
	Status         string                 `json:"status"`
	Payload        db.RawJSON             `json:"payload"`
	Attempts       int                    `json:"attempts"`
	DeliveredAt    *db.PrismaTime         `json:"deliveredAt"`
	FailedAt       *db.PrismaTime         `json:"failedAt"`
	ErrorMessage   *string                `json:"errorMessage"`
	CreatedAt      db.PrismaTime          `json:"createdAt"`
	Subscription   restSubscriptionScalar `json:"subscription"`
	Signal         restSignalTitle        `json:"signal"`
}

func (s *Server) listDeliveries(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	rows, err := s.DB.ListDeliveries(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]restDelivery, len(rows))
	for i, e := range rows {
		out[i] = restDelivery{
			ID: e.ID, SubscriptionID: e.SubscriptionID, SignalID: e.SignalID,
			Channel: e.Channel, Status: e.Status, Payload: e.Payload, Attempts: e.Attempts,
			DeliveredAt: db.NewTimePtr(e.DeliveredAt), FailedAt: db.NewTimePtr(e.FailedAt),
			ErrorMessage: e.ErrorMessage, CreatedAt: db.NewTime(e.CreatedAt),
			Subscription: restSubscriptionScalar{
				ID: e.Subscription.ID, SubscriberID: e.Subscription.SubscriberID, Name: e.Subscription.Name,
				Channel: e.Subscription.Channel, Filter: e.Subscription.Filter, Config: e.Subscription.Config,
				Enabled: e.Subscription.Enabled, CreatedAt: db.NewTime(e.Subscription.CreatedAt),
			},
			Signal: restSignalTitle{Title: e.SignalTitle},
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Data []restDelivery `json:"data"`
	}{out})
}
