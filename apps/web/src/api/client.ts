import type { components, operations } from './schema';

/**
 * The API's own resource types, taken from the generated OpenAPI schema rather
 * than restated here. The schema is generated from the protos, so these cannot
 * drift from what the service actually returns.
 */
export type Company = components['schemas']['Company'];
export type Dividend = components['schemas']['Dividend'];
export type Schedule = components['schemas']['Schedule'];
export type Money = components['schemas']['Money'];
export type GoogleDate = components['schemas']['GoogleTypeDate'];

export type ListDividendsResponse =
  operations['DividendsService_ListDividends']['responses']['200']['content']['application/json'];
export type ListCompaniesResponse =
  operations['DividendsService_ListCompanies']['responses']['200']['content']['application/json'];

/** The AIP-193 error body every failure uses. */
export interface ApiErrorBody {
  error: { code: number; message: string; status: string };
}

/** An error the API reported, carrying its status name and HTTP code. */
export class ApiError extends Error {
  readonly status: string;
  readonly code: number;

  constructor(body: ApiErrorBody) {
    super(body.error.message);
    this.name = 'ApiError';
    this.status = body.error.status;
    this.code = body.error.code;
  }
}

/**
 * Where the API lives. In development the Vite server proxies /v1, so the
 * default of an empty base works both there and when the built client is
 * served from the same host as the API.
 */
const baseUrl = import.meta.env.VITE_API_URL ?? '';

/** Performs a GET against the API, raising ApiError on a reported failure. */
async function get<T>(path: string, params: Record<string, string | number | undefined>): Promise<T> {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') query.set(key, String(value));
  }
  const suffix = query.size > 0 ? `?${query}` : '';

  const response = await fetch(`${baseUrl}${path}${suffix}`, {
    headers: { Accept: 'application/json' },
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as ApiErrorBody | null;
    if (body?.error) throw new ApiError(body);
    throw new Error(`${response.status} ${response.statusText}`);
  }
  return (await response.json()) as T;
}

/** Options accepted by the dividend listing, mirroring the API's parameters. */
export interface ListDividendsOptions {
  /** A company symbol, or undefined to list across every company. */
  company?: string;
  /** An AIP-160 filter expression. */
  filter?: string;
  /** An AIP-132 ordering. */
  orderBy?: string;
  pageSize?: number;
  pageToken?: string;
}

/** Lists dividends, using the AIP-159 wildcard when no company is named. */
export function listDividends(options: ListDividendsOptions): Promise<ListDividendsResponse> {
  const parent = options.company ?? '-';
  return get<ListDividendsResponse>(`/v1/companies/${encodeURIComponent(parent)}/dividends`, {
    filter: options.filter,
    orderBy: options.orderBy,
    pageSize: options.pageSize,
    pageToken: options.pageToken,
  });
}

/** Lists companies known to have announced a dividend. */
export function listCompanies(pageSize = 200): Promise<ListCompaniesResponse> {
  return get<ListCompaniesResponse>('/v1/companies', { pageSize, orderBy: 'symbol' });
}
