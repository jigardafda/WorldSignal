package httpapi

import (
	"context"
	"time"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
)

func strVal(v any) string {
	s, _ := v.(string)
	return s
}

// timePtrT renders a *time.Time as a value (or nil) for the GraphQL DateTime scalar.
func timePtrT(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// registerEntityResolvers wires Phase B resolvers (full entity coverage + analytics).
func (s *Server) registerEntityResolvers(q, m map[string]gql.FieldResolver) {
	q["source"] = s.resolveSource
	q["signalCount"] = s.resolveSignalCount
	q["articles"] = s.resolveArticles
	q["article"] = s.resolveArticle
	q["rawItems"] = s.resolveRawItems
	q["rawItem"] = s.resolveRawItem
	q["deliveries"] = s.resolveDeliveries
	q["delivery"] = s.resolveDelivery
	q["subscribers"] = s.resolveSubscribers
	q["subscription"] = s.resolveSubscription
	q["taxonomyStats"] = s.resolveTaxonomyStats
	q["jobs"] = s.resolveJobs
	q["jobCounts"] = s.resolveJobCounts
	q["analytics"] = s.resolveAnalytics

	m["updateSource"] = s.mutUpdateSource
	m["deleteSource"] = s.mutDeleteSource
	m["revalidateSource"] = s.mutRevalidateSource
	m["updateSubscription"] = s.mutUpdateSubscription
	m["deleteSubscription"] = s.mutDeleteSubscription
	m["createSubscriber"] = s.mutCreateSubscriber
	m["deleteSubscriber"] = s.mutDeleteSubscriber
	m["retryDelivery"] = s.mutRetryDelivery
	m["retryJob"] = s.mutRetryJob
}

func strArg(args map[string]any, key string) *string {
	if v, ok := args[key].(string); ok && v != "" {
		return &v
	}
	return nil
}

func listFilter(args map[string]any) db.ListFilter {
	f := db.ListFilter{Limit: toInt(args["limit"], 50), Offset: toInt(args["offset"], 0)}
	f.SourceID = strArg(args, "sourceId")
	f.Status = strArg(args, "status")
	f.Search = strArg(args, "search")
	return f
}

func buckets(bs []db.Bucket) []any {
	out := make([]any, len(bs))
	for i, b := range bs {
		out[i] = map[string]any{"key": b.Key, "count": b.Count}
	}
	return out
}

// ---- sources ----

func sourceDetailMap(src *db.Source) map[string]any {
	m := sourceToGqlMap(src)
	m["crawlFrequency"] = src.CrawlFrequency
	m["parserType"] = src.ParserType
	m["config"] = src.Config
	m["subcategory"] = src.Subcategory
	m["websiteUrl"] = src.WebsiteURL
	m["contentType"] = src.ContentType
	m["updateFrequency"] = src.UpdateFrequency
	m["biasRating"] = src.BiasRating
	m["avgResponseMs"] = intPtr(src.AvgResponseMs)
	m["lastValidationError"] = src.LastValidationError
	m["metadata"] = src.Metadata
	m["lastFetchedAt"] = timePtr(src.LastFetchedAt)
	m["createdAt"] = src.CreatedAt.Time
	m["updatedAt"] = src.UpdatedAt.Time
	return m
}

func (s *Server) resolveSource(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesRead); err != nil {
		return nil, err
	}
	src, err := s.DB.GetSource(ctx, strVal(args["id"]))
	if err != nil || src == nil {
		return nil, err
	}
	m := sourceDetailMap(src)
	logs, err := s.DB.ListValidationLogs(ctx, src.ID, 50)
	if err != nil {
		return nil, err
	}
	m["validationLogs"] = validationLogMaps(logs)
	return m, nil
}

func validationLogMaps(logs []db.ValidationLog) []any {
	out := make([]any, len(logs))
	for i, l := range logs {
		out[i] = map[string]any{
			"id": l.ID, "checkedAt": l.CheckedAt.Time, "ok": l.OK,
			"httpStatus": intPtr(l.HTTPStatus), "responseMs": intPtr(l.ResponseMs),
			"itemCount": intPtr(l.ItemCount), "newestItemAt": timePtr(l.NewestItemAt),
			"redirectedTo": l.RedirectedTo, "error": l.Error,
		}
	}
	return out
}

func (s *Server) mutUpdateSource(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesWrite); err != nil {
		return nil, err
	}
	input, _ := args["input"].(map[string]any)
	var p db.SourceFullPatch
	if v, ok := input["name"].(string); ok {
		p.Name = &v
	}
	if v, ok := input["country"].(string); ok {
		p.Country = &v
	}
	if v, ok := toFloatOK(input["priority"]); ok {
		n := int(v)
		p.Priority = &n
	}
	if v, ok := toFloatOK(input["credibility"]); ok {
		p.Credibility = &v
	}
	if v, ok := toFloatOK(input["crawlFrequency"]); ok {
		n := int(v)
		p.CrawlFrequency = &n
	}
	if v, ok := input["enabled"].(bool); ok {
		p.Enabled = &v
	}
	src, err := s.DB.UpdateSourceFull(ctx, strVal(args["id"]), p)
	if err != nil || src == nil {
		return nil, err
	}
	return sourceDetailMap(src), nil
}

func (s *Server) mutDeleteSource(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesWrite); err != nil {
		return nil, err
	}
	ok, err := s.DB.DeleteSource(ctx, strVal(args["id"]))
	return ok, err
}

// ---- signals count ----

func (s *Server) resolveSignalCount(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	var f db.SignalFilter
	if filter, ok := args["filter"].(map[string]any); ok {
		f.Country = strArg(filter, "country")
		f.Status = strArg(filter, "status")
		f.Search = strArg(filter, "search")
		if mc, ok := toFloatOK(filter["minConfidence"]); ok {
			f.MinConfidence = &mc
		}
		if tags, ok := filter["tags"].([]any); ok {
			for _, t := range tags {
				if ts, ok := t.(string); ok {
					f.Tags = append(f.Tags, ts)
				}
			}
		}
	}
	return s.DB.CountSignals(ctx, f)
}

// ---- articles ----

func articleRowMap(a db.ArticleRow) map[string]any {
	return map[string]any{
		"id": a.ID, "title": a.Title, "canonicalUrl": a.CanonicalURL, "summary": a.Summary,
		"publishedAt": timePtrT(a.PublishedAt), "fetchedAt": a.FetchedAt,
		"sourceId": a.SourceID, "sourceName": a.SourceName, "signalCount": a.SignalCount,
	}
}

func (s *Server) resolveArticles(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	rows, total, err := s.DB.ListArticles(ctx, listFilter(args))
	if err != nil {
		return nil, err
	}
	items := make([]any, len(rows))
	for i, a := range rows {
		items[i] = articleRowMap(a)
	}
	return map[string]any{"items": items, "total": total}, nil
}

func (s *Server) resolveArticle(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	a, err := s.DB.GetArticle(ctx, strVal(args["id"]))
	if err != nil || a == nil {
		return nil, err
	}
	signals := make([]any, len(a.Signals))
	for i, ls := range a.Signals {
		signals[i] = map[string]any{"id": ls.ID, "title": ls.Title, "relationType": ls.RelationType, "similarityScore": ls.Similarity}
	}
	m := articleRowMap(a.ArticleRow)
	m["body"] = a.Body
	m["author"] = a.Author
	m["language"] = a.Language
	m["country"] = a.Country
	m["contentHash"] = a.ContentHash
	m["tokenSet"] = a.TokenSet
	m["signals"] = signals
	return m, nil
}

// ---- raw items ----

func rawItemRowMap(r db.RawItemRow) map[string]any {
	return map[string]any{
		"id": r.ID, "sourceId": r.SourceID, "sourceName": r.SourceName, "sourceGuid": r.SourceGuid,
		"rawUrl": r.RawURL, "rawTitle": r.RawTitle, "status": r.Status,
		"publishedAt": timePtrT(r.PublishedAt), "fetchedAt": r.FetchedAt,
	}
}

func (s *Server) resolveRawItems(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	rows, total, err := s.DB.ListRawItems(ctx, listFilter(args))
	if err != nil {
		return nil, err
	}
	items := make([]any, len(rows))
	for i, r := range rows {
		items[i] = rawItemRowMap(r)
	}
	return map[string]any{"items": items, "total": total}, nil
}

func (s *Server) resolveRawItem(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	r, err := s.DB.GetRawItemDetail(ctx, strVal(args["id"]))
	if err != nil || r == nil {
		return nil, err
	}
	m := rawItemRowMap(r.RawItemRow)
	m["rawContent"] = r.RawContent
	m["contentHash"] = r.ContentHash
	m["rawPayload"] = r.RawPayload
	return m, nil
}

// ---- deliveries ----

func deliveryRowMap(r db.DeliveryRow) map[string]any {
	return map[string]any{
		"id": r.ID, "subscriptionId": r.SubscriptionID, "subscriptionName": r.SubscriptionName,
		"channel": r.Channel, "signalId": r.SignalID, "signalTitle": r.SignalTitle,
		"status": r.Status, "attempts": r.Attempts, "createdAt": r.CreatedAt,
		"deliveredAt": timePtrT(r.DeliveredAt), "failedAt": timePtrT(r.FailedAt), "errorMessage": r.ErrorMessage,
	}
}

func (s *Server) resolveDeliveries(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermDeliveriesRead); err != nil {
		return nil, err
	}
	f := db.DeliveryFilter{Limit: toInt(args["limit"], 50), Offset: toInt(args["offset"], 0),
		Status: strArg(args, "status"), SubscriptionID: strArg(args, "subscriptionId")}
	rows, total, err := s.DB.ListDeliveriesFiltered(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]any, len(rows))
	for i, r := range rows {
		items[i] = deliveryRowMap(r)
	}
	return map[string]any{"items": items, "total": total}, nil
}

func (s *Server) resolveDelivery(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermDeliveriesRead); err != nil {
		return nil, err
	}
	r, err := s.DB.GetDeliveryDetail(ctx, strVal(args["id"]))
	if err != nil || r == nil {
		return nil, err
	}
	m := deliveryRowMap(r.DeliveryRow)
	m["payload"] = r.Payload
	return m, nil
}

func (s *Server) mutRetryDelivery(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermDeliveriesRetry); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	ok, err := s.DB.ResetDeliveryForRetry(ctx, id)
	if err != nil || !ok {
		return ok, err
	}
	if err := s.Enqueue.EnqueueSendDelivery(id); err != nil {
		return nil, err
	}
	return true, nil
}

// ---- subscribers / subscriptions ----

func subscriberMap(s db.Subscriber) map[string]any {
	return map[string]any{"id": s.ID, "name": s.Name, "status": s.Status, "createdAt": s.CreatedAt, "subscriptionCount": s.SubscriptionCount}
}

func (s *Server) resolveSubscribers(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsRead); err != nil {
		return nil, err
	}
	subs, err := s.DB.ListSubscribers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(subs))
	for i, sub := range subs {
		out[i] = subscriberMap(sub)
	}
	return out, nil
}

func (s *Server) resolveSubscription(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsRead); err != nil {
		return nil, err
	}
	subs, err := s.DB.ListSubscriptionsBasic(ctx)
	if err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	for _, sub := range subs {
		if sub.ID == id {
			return map[string]any{
				"id": sub.ID, "subscriberId": sub.SubscriberID, "name": sub.Name, "channel": sub.Channel,
				"enabled": sub.Enabled, "filter": sub.Filter, "config": sub.Config, "createdAt": sub.CreatedAt,
			}, nil
		}
	}
	return nil, nil
}

func (s *Server) mutUpdateSubscription(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsWrite); err != nil {
		return nil, err
	}
	input, _ := args["input"].(map[string]any)
	var p db.SubscriptionPatch
	if v, ok := input["name"].(string); ok {
		p.Name = &v
	}
	if v, ok := input["enabled"].(bool); ok {
		p.Enabled = &v
	}
	if v, ok := input["filter"]; ok && v != nil {
		p.Filter = jsonRaw(v)
	}
	if v, ok := input["config"]; ok && v != nil {
		p.Config = jsonRaw(v)
	}
	sub, err := s.DB.UpdateSubscription(ctx, strVal(args["id"]), p)
	if err != nil || sub == nil {
		return nil, err
	}
	return map[string]any{
		"id": sub.ID, "subscriberId": sub.SubscriberID, "name": sub.Name, "channel": sub.Channel,
		"enabled": sub.Enabled, "filter": sub.Filter, "config": sub.Config, "createdAt": sub.CreatedAt,
	}, nil
}

func (s *Server) mutDeleteSubscription(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsWrite); err != nil {
		return nil, err
	}
	return s.DB.DeleteSubscription(ctx, strVal(args["id"]))
}

func (s *Server) mutCreateSubscriber(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsWrite); err != nil {
		return nil, err
	}
	name := strVal(args["name"])
	if name == "" {
		return nil, errValidation
	}
	sub, err := s.DB.CreateSubscriber(ctx, name)
	if err != nil {
		return nil, err
	}
	return subscriberMap(*sub), nil
}

func (s *Server) mutDeleteSubscriber(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsWrite); err != nil {
		return nil, err
	}
	return s.DB.DeleteSubscriber(ctx, strVal(args["id"]))
}

// ---- taxonomy stats ----

func (s *Server) resolveTaxonomyStats(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	counts, err := s.DB.TaxonomyCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(counts))
	for code, n := range counts {
		out = append(out, map[string]any{"code": code, "count": n})
	}
	return out, nil
}

// ---- jobs ----

func (s *Server) resolveJobs(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermJobsRead); err != nil {
		return nil, err
	}
	f := db.JobFilter{Limit: toInt(args["limit"], 50), Offset: toInt(args["offset"], 0),
		Queue: strArg(args, "queue"), State: strArg(args, "state")}
	rows, total, err := s.DB.ListJobs(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]any, len(rows))
	for i, j := range rows {
		items[i] = map[string]any{
			"id": j.ID, "queue": j.Queue, "state": j.State, "retryCount": j.RetryCount, "retryLimit": j.RetryLimit,
			"createdAt": j.CreatedAt, "startedAt": timePtrT(j.StartedAt), "completedAt": timePtrT(j.CompletedAt), "lastError": j.LastError,
		}
	}
	return map[string]any{"items": items, "total": total}, nil
}

func (s *Server) resolveJobCounts(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermJobsRead); err != nil {
		return nil, err
	}
	bs, err := s.DB.JobStateCounts(ctx)
	if err != nil {
		return nil, err
	}
	return buckets(bs), nil
}

func (s *Server) mutRetryJob(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermJobsManage); err != nil {
		return nil, err
	}
	return s.DB.RetryJob(ctx, strVal(args["id"]))
}

// ---- analytics ----

func (s *Server) resolveAnalytics(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermAnalyticsRead); err != nil {
		return nil, err
	}
	sev, err := s.DB.SignalsBySeverity(ctx)
	if err != nil {
		return nil, err
	}
	status, err := s.DB.SignalsByStatus(ctx)
	if err != nil {
		return nil, err
	}
	evt, err := s.DB.SignalsByEventType(ctx, 10)
	if err != nil {
		return nil, err
	}
	country, err := s.DB.SignalsByCountry(ctx, 10)
	if err != nil {
		return nil, err
	}
	overTime, err := s.DB.SignalsOverTime(ctx, 30)
	if err != nil {
		return nil, err
	}
	top, err := s.DB.TopSources(ctx, 10)
	if err != nil {
		return nil, err
	}
	delStats, err := s.DB.GetDeliveryStats(ctx)
	if err != nil {
		return nil, err
	}
	ing, err := s.DB.GetIngestionStats(ctx)
	if err != nil {
		return nil, err
	}
	topSources := make([]any, len(top))
	for i, t := range top {
		topSources[i] = map[string]any{"id": t.ID, "name": t.Name, "articleCount": t.ArticleCount}
	}
	return map[string]any{
		"signalsBySeverity":  buckets(sev),
		"signalsByStatus":    buckets(status),
		"signalsByEventType": buckets(evt),
		"signalsByCountry":   buckets(country),
		"signalsOverTime":    buckets(overTime),
		"topSources":         topSources,
		"deliveryStats": map[string]any{
			"total": delStats.Total, "sent": delStats.Sent, "pending": delStats.Pending,
			"retrying": delStats.Retrying, "failed": delStats.Failed, "deadLettered": delStats.DeadLettered,
		},
		"ingestionStats": map[string]any{
			"rawItems": ing.RawItems, "parsed": ing.Parsed, "duplicates": ing.Duplicates,
			"failed": ing.Failed, "articles": ing.Articles,
		},
	}, nil
}
