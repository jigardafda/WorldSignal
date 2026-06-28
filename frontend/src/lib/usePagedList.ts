import { useState } from "react";
import type { Page } from "./api";
import { useAsync, type AsyncState } from "./useAsync";

export const PAGE_SIZE = 25;

/** Manages page state for a paginated resource. Reset `page` to 1 when filters
 * change by including them in `deps`. */
export function usePagedList<T>(
  fetchPage: (limit: number, offset: number) => Promise<Page<T>>,
  deps: unknown[],
): { state: AsyncState<Page<T>>; page: number; setPage: (p: number) => void; totalPages: number } {
  const [page, setPage] = useState(1);
  const state = useAsync<Page<T>>(() => fetchPage(PAGE_SIZE, (page - 1) * PAGE_SIZE), [...deps, page]);
  const totalPages = Math.max(1, Math.ceil((state.data?.total ?? 0) / PAGE_SIZE));
  return { state, page, setPage, totalPages };
}
