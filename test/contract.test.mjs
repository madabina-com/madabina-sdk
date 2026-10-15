import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('the public Laravel OpenAPI document is byte-for-byte aligned with the SDK contract', async () => {
  const sdk = await readFile(new URL('../openapi.json', import.meta.url), 'utf8');
  const laravel = await readFile(new URL('../../web-app/resources/openapi/madabina-rest.json', import.meta.url), 'utf8');
  assert.equal(JSON.stringify(JSON.parse(laravel)), JSON.stringify(JSON.parse(sdk)));
});

test('OpenAPI documents every current REST v1 route and method', async () => {
  const spec = JSON.parse(await readFile(new URL('../openapi.json', import.meta.url), 'utf8'));
  const expected = {
    '/auth/login': ['post'], '/auth/refresh': ['post'], '/auth/me': ['get'], '/auth/logout': ['post'],
    '/stats/overview': ['get'], '/notifications/endpoints': ['get', 'post', 'delete'],
    '/products': ['get', 'post'], '/products/{uuid}': ['get', 'put', 'patch', 'delete'],
    '/products/{uuid}/stock': ['patch'], '/products/{uuid}/visibility': ['patch'],
    '/orders': ['get', 'post'], '/orders/{uuid}': ['get', 'delete'],
    '/orders/{uuid}/status': ['patch'], '/orders/{uuid}/tracking': ['patch'],
    '/orders/{uuid}/payment-status': ['patch'], '/customers': ['get'], '/customers/{uuid}': ['get'],
  };

  for (const [path, methods] of Object.entries(expected)) {
    for (const method of methods) assert.ok(spec.paths[path]?.[method], `Missing ${method.toUpperCase()} ${path}`);
  }
});

test('OpenAPI component references resolve and schema types use OpenAPI 3.0 syntax', async () => {
  const spec = JSON.parse(await readFile(new URL('../openapi.json', import.meta.url), 'utf8'));
  const references = [];
  const schemaTypeArrays = [];
  const walk = (value, location = '$') => {
    if (Array.isArray(value)) return value.forEach((item, index) => walk(item, `${location}[${index}]`));
    if (!value || typeof value !== 'object') return;
    if (typeof value.$ref === 'string' && value.$ref.startsWith('#/components/schemas/')) references.push([location, value.$ref.split('/').at(-1)]);
    if (Array.isArray(value.type)) schemaTypeArrays.push(location);
    for (const [key, child] of Object.entries(value)) walk(child, `${location}.${key}`);
  };
  walk(spec);
  for (const [location, name] of references) assert.ok(spec.components.schemas[name], `${location} references missing schema ${name}`);
  assert.deepEqual(schemaTypeArrays, []);
  assert.ok(spec.components.schemas.PaginationMeta.properties.lastPage);
});
