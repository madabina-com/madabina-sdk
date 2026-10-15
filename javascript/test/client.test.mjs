import test from 'node:test';
import assert from 'node:assert/strict';
import { MadabinaApiError, MadabinaClient } from '../src/index.js';

const json = (status, body) => new Response(JSON.stringify(body), {
  status,
  headers: { 'content-type': 'application/json' },
});

test('refreshes once after 401 and returns normalized pagination', async () => {
  const calls = [];
  const responses = [
    json(401, { success: false, message: 'Expired', statusCode: 401 }),
    json(200, { success: true, data: { accessToken: 'new-access', refreshToken: 'new-refresh' } }),
    json(200, { success: true, data: { products: [{ uuid: 'p-1' }] }, meta: { page: 2, perPage: 10, total: 11, lastPage: 2 } }),
  ];
  const client = new MadabinaClient({
    baseUrl: 'https://example.test/api/v1/',
    accessToken: 'old-access',
    refreshToken: 'old-refresh',
    fetch: async (url, init) => { calls.push({ url: String(url), init }); return responses.shift(); },
  });

  const page = await client.products.list({ search: 'blue phone', isVisible: true });
  assert.deepEqual(page, { items: [{ uuid: 'p-1' }], page: 2, perPage: 10, total: 11, lastPage: 2 });
  assert.equal(calls.length, 3);
  assert.equal(calls[0].init.headers.get('authorization'), 'Bearer old-access');
  assert.match(calls[1].url, /\/auth\/refresh$/);
  assert.equal(calls[2].init.headers.get('authorization'), 'Bearer new-access');
  assert.match(calls[2].url, /search=blue\+phone/);
  assert.match(calls[2].url, /isVisible=true/);
  assert.deepEqual(client.getTokens(), { accessToken: 'new-access', refreshToken: 'new-refresh' });
});

test('represents API failures with status and validation details', async () => {
  const client = new MadabinaClient({
    fetch: async () => json(422, { success: false, message: 'Invalid data', statusCode: 422, details: { email: ['Required'] } }),
  });

  await assert.rejects(client.auth.login({ email: '', password: '' }), (error) => {
    assert.ok(error instanceof MadabinaApiError);
    assert.equal(error.status, 422);
    assert.deepEqual(error.details, { email: ['Required'] });
    return true;
  });
});

test('handles 204 deletes without trying to parse a response body', async () => {
  const client = new MadabinaClient({ accessToken: 'token', fetch: async () => new Response(null, { status: 204 }) });
  await assert.doesNotReject(client.products.delete('product/1'));
});

test('exposes both PUT and PATCH product update routes', async () => {
  const calls = [];
  const client = new MadabinaClient({
    accessToken: 'token',
    fetch: async (url, init) => {
      calls.push({ url: String(url), method: init.method });
      return json(200, { success: true, data: { product: { uuid: 'p-1' } }, meta: {} });
    },
  });
  await client.products.update('p-1', { titleFr: 'Nouveau' });
  await client.products.patch('p-1', { titleFr: 'Nouveau' });
  assert.deepEqual(calls.map(({ method, url }) => [method, url]), [
    ['PUT', 'https://madabina.com/api/v1/products/p-1'],
    ['PATCH', 'https://madabina.com/api/v1/products/p-1'],
  ]);
});

test('provides a callable for every documented REST v1 operation', async () => {
  const calls = [];
  const client = new MadabinaClient({
    baseUrl: 'https://example.test/api/v1',
    fetch: async (url, init) => {
      const path = new URL(String(url)).pathname.replace('/api/v1', '');
      calls.push(`${init.method} ${path}`);
      return json(200, {
        success: true,
        data: {
          accessToken: 'access', refreshToken: 'refresh', user: { uuid: 'u-1' },
          products: [], product: { uuid: 'p-1' }, orders: [], order: { uuid: 'o-1' },
          customers: [], customer: { uuid: 'c-1' }, endpoints: [], endpoint: { id: 1 },
        },
        meta: { page: 1, perPage: 15, total: 0, lastPage: 1 },
      });
    },
  });

  await client.auth.login({ email: 'a@example.com', password: 'secret' });
  await client.auth.refresh();
  await client.auth.me();
  await client.auth.logout();
  await client.stats.overview();
  await client.notifications.listEndpoints();
  await client.notifications.registerEndpoint({ channel: 'web_push', endpoint: 'endpoint' });
  await client.notifications.removeEndpoint({ channel: 'web_push', endpoint: 'endpoint' });
  await client.products.list();
  await client.products.create({ titleFr: 'x' });
  await client.products.get('p-1');
  await client.products.update('p-1', { titleFr: 'x' });
  await client.products.patch('p-1', { titleFr: 'x' });
  await client.products.updateStock('p-1', { delta: 1 });
  await client.products.updateVisibility('p-1', true);
  await client.products.delete('p-1');
  await client.orders.list();
  await client.orders.create({ customer: {}, items: [] });
  await client.orders.get('o-1');
  await client.orders.cancel('o-1');
  await client.orders.updateStatus('o-1', 'SHIPPED');
  await client.orders.updateTracking('o-1', { carrier: 'Carrier' });
  await client.orders.updatePaymentStatus('o-1', 'PAID');
  await client.customers.list();
  await client.customers.get('c-1');

  assert.deepEqual(calls, [
    'POST /auth/login', 'POST /auth/refresh', 'GET /auth/me', 'POST /auth/logout',
    'GET /stats/overview', 'GET /notifications/endpoints', 'POST /notifications/endpoints', 'DELETE /notifications/endpoints',
    'GET /products', 'POST /products', 'GET /products/p-1', 'PUT /products/p-1', 'PATCH /products/p-1',
    'PATCH /products/p-1/stock', 'PATCH /products/p-1/visibility', 'DELETE /products/p-1',
    'GET /orders', 'POST /orders', 'GET /orders/o-1', 'DELETE /orders/o-1', 'PATCH /orders/o-1/status',
    'PATCH /orders/o-1/tracking', 'PATCH /orders/o-1/payment-status', 'GET /customers', 'GET /customers/c-1',
  ]);
});

test('shares one token refresh between simultaneous expired requests', async () => {
  let oldTokenRequests = 0;
  let refreshRequests = 0;
  let releaseBoth;
  const bothOldRequests = new Promise((resolve) => { releaseBoth = resolve; });
  const client = new MadabinaClient({
    accessToken: 'old-access',
    refreshToken: 'refresh',
    fetch: async (url, init) => {
      const path = new URL(String(url)).pathname;
      if (path.endsWith('/auth/refresh')) {
        refreshRequests += 1;
        return json(200, { success: true, data: { accessToken: 'new-access', refreshToken: 'new-refresh' } });
      }
      if (init.headers.get('authorization') === 'Bearer old-access') {
        oldTokenRequests += 1;
        if (oldTokenRequests === 2) releaseBoth();
        await bothOldRequests;
        return json(401, { success: false, message: 'Expired' });
      }
      return json(200, { success: true, data: { product: { uuid: path.split('/').at(-1) } } });
    },
  });

  await Promise.all([client.products.get('p-1'), client.products.get('p-2')]);
  assert.equal(refreshRequests, 1);
  assert.equal(oldTokenRequests, 2);
});

test('clears stored credentials when refresh is rejected', async () => {
  const client = new MadabinaClient({
    accessToken: 'expired-access',
    refreshToken: 'revoked-refresh',
    fetch: async () => json(401, { success: false, message: 'Invalid refresh token' }),
  });

  await assert.rejects(client.auth.refresh(), (error) => error.status === 401);
  assert.deepEqual(client.getTokens(), {});
});
