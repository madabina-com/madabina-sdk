const DEFAULT_BASE_URL = 'https://madabina.com/api/v1';

export class MadabinaApiError extends Error {
  constructor(message, status, details = undefined) {
    super(message);
    this.name = 'MadabinaApiError';
    this.status = status;
    this.details = details;
  }
}

class Resource {
  constructor(client) {
    this.client = client;
  }

  async page(path, key, query = {}) {
    const result = await this.client.request(`${path}${queryString(query)}`);
    const items = result?.data?.[key] ?? [];
    const meta = result?.meta ?? {};
    return {
      items,
      page: Number(meta.page ?? 1),
      perPage: Number(meta.perPage ?? items.length),
      total: Number(meta.total ?? items.length),
      lastPage: Number(meta.lastPage ?? 1),
    };
  }
}

class Authentication extends Resource {
  async login(credentials) {
    this.client.clearTokens();
    const result = await this.client.request('/auth/login', { method: 'POST', body: credentials });
    const { accessToken, refreshToken, expiresIn, tokenType, user } = result.data;
    const tokens = { accessToken, refreshToken, ...(expiresIn === undefined ? {} : { expiresIn }), ...(tokenType === undefined ? {} : { tokenType }) };
    this.client.setTokens(tokens);
    return { tokens, user };
  }

  async refresh() {
    const refreshToken = this.client.getTokens().refreshToken;
    if (!refreshToken) throw new MadabinaApiError('No refresh token is available.', 401);
    try {
      const result = await this.client.request('/auth/refresh', { method: 'POST', body: { refreshToken } }, { retry: false });
      this.client.setTokens(result.data);
      return this.client.getTokens();
    } catch (error) {
      this.client.clearTokens();
      throw error;
    }
  }

  async me() {
    return (await this.client.request('/auth/me')).data.user;
  }

  async logout() {
    const refreshToken = this.client.getTokens().refreshToken;
    if (!refreshToken) {
      this.client.clearTokens();
      return;
    }
    try {
      await this.client.request('/auth/logout', { method: 'POST', body: { refreshToken } }, { retry: false });
    } finally {
      this.client.clearTokens();
    }
  }
}

class Products extends Resource {
  list(query) { return this.page('/products', 'products', query); }
  async get(uuid) { return (await this.client.request(`/products/${encodeURIComponent(uuid)}`)).data.product; }
  async create(payload) { return (await this.client.request('/products', { method: 'POST', body: payload })).data.product; }
  async update(uuid, payload) { return (await this.client.request(`/products/${encodeURIComponent(uuid)}`, { method: 'PUT', body: payload })).data.product; }
  async patch(uuid, payload) { return (await this.client.request(`/products/${encodeURIComponent(uuid)}`, { method: 'PATCH', body: payload })).data.product; }
  async updateStock(uuid, payload) { return (await this.client.request(`/products/${encodeURIComponent(uuid)}/stock`, { method: 'PATCH', body: payload })).data.product; }
  async updateVisibility(uuid, isVisible) { return (await this.client.request(`/products/${encodeURIComponent(uuid)}/visibility`, { method: 'PATCH', body: { isVisible } })).data.product; }
  async delete(uuid) { await this.client.request(`/products/${encodeURIComponent(uuid)}`, { method: 'DELETE' }); }
}

class Orders extends Resource {
  list(query) { return this.page('/orders', 'orders', query); }
  async get(uuid) { return (await this.client.request(`/orders/${encodeURIComponent(uuid)}`)).data.order; }
  async create(payload) { return (await this.client.request('/orders', { method: 'POST', body: payload })).data.order; }
  async updateStatus(uuid, status) { return (await this.client.request(`/orders/${encodeURIComponent(uuid)}/status`, { method: 'PATCH', body: { status } })).data.order; }
  async updateTracking(uuid, payload) { return (await this.client.request(`/orders/${encodeURIComponent(uuid)}/tracking`, { method: 'PATCH', body: payload })).data.order; }
  async updatePaymentStatus(uuid, paymentStatus) { return (await this.client.request(`/orders/${encodeURIComponent(uuid)}/payment-status`, { method: 'PATCH', body: { paymentStatus } })).data.order; }
  async cancel(uuid) { return (await this.client.request(`/orders/${encodeURIComponent(uuid)}`, { method: 'DELETE' })).data.order; }
}

class Customers extends Resource {
  list(query) { return this.page('/customers', 'customers', query); }
  async get(uuid) { return (await this.client.request(`/customers/${encodeURIComponent(uuid)}`)).data.customer; }
}

class Statistics extends Resource {
  async overview() { return (await this.client.request('/stats/overview')).data; }
}

class Notifications extends Resource {
  async listEndpoints() { return (await this.client.request('/notifications/endpoints')).data.endpoints; }
  async registerEndpoint(payload) { return (await this.client.request('/notifications/endpoints', { method: 'POST', body: payload })).data.endpoint; }
  async removeEndpoint(payload) { await this.client.request('/notifications/endpoints', { method: 'DELETE', body: payload }); }
}

export class MadabinaClient {
  #accessToken;
  #refreshToken;
  #refreshInFlight;

  constructor({ baseUrl = DEFAULT_BASE_URL, accessToken, refreshToken, fetch: fetchImpl = globalThis.fetch } = {}) {
    if (typeof fetchImpl !== 'function') throw new TypeError('A Fetch-compatible implementation is required.');
    this.baseUrl = baseUrl.replace(/\/+$/, '');
    this.fetch = fetchImpl;
    this.#accessToken = accessToken;
    this.#refreshToken = refreshToken;
    this.auth = new Authentication(this);
    this.products = new Products(this);
    this.orders = new Orders(this);
    this.customers = new Customers(this);
    this.stats = new Statistics(this);
    this.notifications = new Notifications(this);
  }

  getTokens() {
    return {
      ...(this.#accessToken ? { accessToken: this.#accessToken } : {}),
      ...(this.#refreshToken ? { refreshToken: this.#refreshToken } : {}),
    };
  }

  setTokens({ accessToken, refreshToken }) {
    this.#accessToken = accessToken;
    this.#refreshToken = refreshToken;
  }

  clearTokens() {
    this.#accessToken = undefined;
    this.#refreshToken = undefined;
  }

  async request(path, { method = 'GET', body, headers: suppliedHeaders } = {}, { retry = true } = {}) {
    const headers = new Headers(suppliedHeaders);
    headers.set('Accept', 'application/json');
    if (body !== undefined) headers.set('Content-Type', 'application/json');
    if (this.#accessToken) headers.set('Authorization', `Bearer ${this.#accessToken}`);
    const response = await this.fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });

    const authEndpoint = ['/auth/login', '/auth/refresh', '/auth/logout'].includes(path);
    if (response.status === 401 && retry && !authEndpoint && this.#refreshToken) {
      this.#refreshInFlight ??= this.auth.refresh().then(() => undefined).finally(() => { this.#refreshInFlight = undefined; });
      await this.#refreshInFlight;
      return this.request(path, { method, body, headers: suppliedHeaders }, { retry: false });
    }

    const payload = response.status === 204 ? undefined : await response.json().catch(() => undefined);
    if (!response.ok) {
      throw new MadabinaApiError(payload?.message ?? `Madabina API request failed (${response.status}).`, response.status, payload?.details);
    }
    if (payload && typeof payload === 'object' && 'success' in payload) {
      if (!payload.success) throw new MadabinaApiError(payload.message ?? 'Madabina API request failed.', response.status, payload.details);
      return { data: payload.data ?? {}, meta: payload.meta ?? {} };
    }
    return { data: payload, meta: {} };
  }
}

function queryString(query = {}) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null) params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : '';
}
