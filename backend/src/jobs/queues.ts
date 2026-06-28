// Central registry of pg-boss queue names + job payload types.
export const QUEUES = {
  fetchSource: "source.fetch",
  processArticle: "article.process",
  enrichSignal: "signal.enrich",
  matchSignal: "signal.match",
  sendDelivery: "delivery.send",
} as const;

export interface FetchSourceJob {
  sourceId: string;
}
export interface ProcessArticleJob {
  rawItemId: string;
}
export interface EnrichSignalJob {
  signalId: string;
}
export interface MatchSignalJob {
  signalId: string;
}
export interface SendDeliveryJob {
  deliveryId: string;
}
