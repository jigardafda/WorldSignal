// IndexedDB cache of the live-signal feed so the map paints instantly on open
// or reload, then reconciles with the network. Data is per-browser and cleared
// on logout. Every operation degrades to a safe no-op when IndexedDB is
// unavailable (e.g. jsdom, private-mode failures), so callers never need guards.

import type { LiveSignal } from "./api";

const DB_NAME = "worldsignal-live";
const STORE = "signals";
const TS_INDEX = "ts";
const MAX_AGE_MS = 24 * 60 * 60_000; // widest window we show
const MAX_ROWS = 5000; // hard ceiling; oldest evicted beyond this

// Stored shape: the feed record plus a numeric timestamp for range/eviction.
type Stored = LiveSignal & { _ts: number };

function idb(): IDBFactory | null {
  return typeof indexedDB === "undefined" ? null : indexedDB;
}

function promisify<T>(req: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

function openDB(): Promise<IDBDatabase | null> {
  const factory = idb();
  if (!factory) return Promise.resolve(null);
  return new Promise((resolve) => {
    let req: IDBOpenDBRequest;
    try {
      req = factory.open(DB_NAME, 1);
    } catch {
      resolve(null);
      return;
    }
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: "id" });
        store.createIndex(TS_INDEX, "_ts");
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => resolve(null); // treat open failure as "no cache"
  });
}

function tsOf(s: LiveSignal): number {
  const t = s.lastSeenAt ? Date.parse(s.lastSeenAt) : NaN;
  return Number.isNaN(t) ? 0 : t;
}

/** Cached records last seen at/after `sinceMs`, newest first. Empty if no cache. */
export async function getCached(sinceMs: number): Promise<LiveSignal[]> {
  const db = await openDB();
  if (!db) return [];
  try {
    const store = db.transaction(STORE, "readonly").objectStore(STORE);
    const range = IDBKeyRange.lowerBound(sinceMs);
    const rows = (await promisify(store.index(TS_INDEX).getAll(range))) as Stored[];
    rows.sort((a, b) => b._ts - a._ts);
    return rows.map(({ _ts, ...rest }) => { void _ts; return rest; });
  } catch {
    return [];
  } finally {
    db.close();
  }
}

/** Upsert the given records (keyed by id), then evict aged-out / overflow rows. */
export async function mergeCached(records: LiveSignal[]): Promise<void> {
  const db = await openDB();
  if (!db) return;
  try {
    const tx = db.transaction(STORE, "readwrite");
    const store = tx.objectStore(STORE);
    for (const r of records) {
      const stored: Stored = { ...r, _ts: tsOf(r) };
      store.put(stored);
    }
    await txDone(tx);
    await evict(db);
  } catch {
    /* best-effort cache; ignore write failures */
  } finally {
    db.close();
  }
}

/** Drop everything (called on logout). */
export async function clearCache(): Promise<void> {
  const db = await openDB();
  if (!db) return;
  try {
    const tx = db.transaction(STORE, "readwrite");
    tx.objectStore(STORE).clear();
    await txDone(tx);
  } catch {
    /* ignore */
  } finally {
    db.close();
  }
}

function txDone(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

// Remove records older than MAX_AGE_MS, and if still over MAX_ROWS, delete the
// oldest until at the ceiling. Uses the ts index cursor from oldest upward.
async function evict(db: IDBDatabase): Promise<void> {
  const cutoff = Date.now() - MAX_AGE_MS;
  const tx = db.transaction(STORE, "readwrite");
  const index = tx.objectStore(STORE).index(TS_INDEX);
  const total = await promisify(index.count());
  let toDelete = Math.max(0, total - MAX_ROWS);
  await new Promise<void>((resolve, reject) => {
    const req = index.openCursor(); // ascending by _ts (oldest first)
    req.onsuccess = () => {
      const cursor = req.result;
      if (!cursor) return resolve();
      const stored = cursor.value as Stored;
      if (stored._ts < cutoff || toDelete > 0) {
        cursor.delete();
        if (toDelete > 0) toDelete--;
        cursor.continue();
      } else {
        // Past the age cutoff and within the row cap ⇒ nothing older remains.
        resolve();
      }
    };
    req.onerror = () => reject(req.error);
  });
  await txDone(tx);
}
