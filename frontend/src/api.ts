export class ApiError extends Error {
  status: number;
  fields?: Record<string, string>;
  constructor(message: string, status: number, fields?: Record<string, string>) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.fields = fields;
  }
}

type ApiErrorShape = { message?: string; fields?: Record<string, string> };

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  headers.set('Accept', 'application/json');
  const method = (init.method ?? 'GET').toUpperCase();
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const csrf = sessionStorage.getItem('orca-csrf-token');
    if (csrf) headers.set('X-CSRF-Token', csrf);
  }
  const response = await fetch(`/api/v1${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 204) return undefined as T;
  const text = await response.text();
  let body: unknown;
  try { body = text ? JSON.parse(text) : undefined; } catch { body = text; }
  if (body && typeof body === 'object') {
    const record = body as Record<string, unknown>;
    const session = record.session && typeof record.session === 'object' ? record.session as Record<string, unknown> : undefined;
    const csrf = record.csrfToken ?? session?.csrfToken;
    if (typeof csrf === 'string' && csrf) sessionStorage.setItem('orca-csrf-token', csrf);
  }
  if (!response.ok) {
    const error = (body ?? {}) as ApiErrorShape;
    throw new ApiError(error.message ?? `The request could not be completed (${response.status}).`, response.status, error.fields);
  }
  return body as T;
}

export async function apiAvailable(): Promise<boolean> {
  try {
    await apiRequest('/health');
    return true;
  } catch (error) {
    // Any HTTP response means the API is reachable. A protected or misrouted
    // health request must never unlock writable local demo mode.
    return error instanceof ApiError;
  }
}

export async function readItemsResult<T>(path: string): Promise<{ items: T[] | null; status?: number }> {
  try {
    const result = await apiRequest<unknown>(path);
    if (Array.isArray(result)) return { items: result as T[] };
    if (result && typeof result === 'object') {
      const record = result as Record<string, unknown>;
      if (Array.isArray(record.items)) return { items: record.items as T[] };
      if (Array.isArray(record.data)) return { items: record.data as T[] };
    }
    return { items: null };
  } catch (error) {
    return { items: null, ...(error instanceof ApiError ? { status: error.status } : {}) };
  }
}

export async function readItems<T>(path: string): Promise<T[] | null> {
  return (await readItemsResult<T>(path)).items;
}

export function post<T>(path: string, body: unknown): Promise<T> {
  return apiRequest<T>(path, { method: 'POST', body: JSON.stringify(body) });
}
