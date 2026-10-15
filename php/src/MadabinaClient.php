<?php

declare(strict_types=1);

namespace Madabina\Sdk;

use JsonException;
use RuntimeException;

final class MadabinaApiException extends RuntimeException
{
    public function __construct(string $message, public readonly int $status, public readonly mixed $details = null)
    {
        parent::__construct($message);
    }
}

final class HttpResponse
{
    public function __construct(public readonly int $status, public readonly string $body)
    {
    }
}

interface TransportInterface
{
    /** @param array<string, string> $headers */
    public function request(string $method, string $url, array $headers, ?string $body): HttpResponse;
}

final class NativeHttpTransport implements TransportInterface
{
    public function __construct(private readonly int $timeoutSeconds = 30)
    {
    }

    public function request(string $method, string $url, array $headers, ?string $body): HttpResponse
    {
        $headerLines = [];
        foreach ($headers as $name => $value) {
            $headerLines[] = $name.': '.$value;
        }
        $context = stream_context_create(['http' => [
            'method' => $method,
            'header' => implode("\r\n", $headerLines),
            'content' => $body ?? '',
            'timeout' => $this->timeoutSeconds,
            'ignore_errors' => true,
        ]]);
        $responseBody = @file_get_contents($url, false, $context);
        $responseHeaders = $http_response_header ?? [];
        $status = 0;
        if (isset($responseHeaders[0]) && preg_match('/\s(\d{3})(?:\s|$)/', $responseHeaders[0], $matches)) {
            $status = (int) $matches[1];
        }
        if ($status === 0) {
            throw new RuntimeException('Could not connect to the Madabina API.');
        }

        return new HttpResponse($status, $responseBody === false ? '' : $responseBody);
    }
}

abstract class Resource
{
    public function __construct(protected readonly MadabinaClient $client)
    {
    }

    /** @return array{items: list<array<string, mixed>>, page: int, perPage: int, total: int, lastPage: int} */
    protected function page(string $path, string $key, array $query): array
    {
        $result = $this->client->request('GET', $path, $query);
        $items = $result['data'][$key] ?? [];
        $meta = $result['meta'] ?? [];

        return [
            'items' => $items,
            'page' => (int) ($meta['page'] ?? 1),
            'perPage' => (int) ($meta['perPage'] ?? count($items)),
            'total' => (int) ($meta['total'] ?? count($items)),
            'lastPage' => (int) ($meta['lastPage'] ?? 1),
        ];
    }
}

final class AuthResource extends Resource
{
    /** @return array{tokens: array<string, mixed>, user: array<string, mixed>} */
    public function login(array $credentials): array
    {
        $this->client->clearTokens();
        $data = $this->client->request('POST', '/auth/login', [], $credentials, retry: false)['data'];
        $tokens = array_intersect_key($data, array_flip(['accessToken', 'refreshToken', 'expiresIn', 'tokenType']));
        $this->client->setTokens($tokens);

        return ['tokens' => $tokens, 'user' => $data['user'] ?? []];
    }

    public function refresh(): array
    {
        $refreshToken = $this->client->getTokens()['refreshToken'] ?? null;
        if (! $refreshToken) {
            throw new MadabinaApiException('No refresh token is available.', 401);
        }
        try {
            $tokens = $this->client->request('POST', '/auth/refresh', [], ['refreshToken' => $refreshToken], retry: false)['data'];
            $this->client->setTokens($tokens);

            return $this->client->getTokens();
        } catch (MadabinaApiException $error) {
            $this->client->clearTokens();
            throw $error;
        }
    }

    public function me(): array
    {
        return $this->client->request('GET', '/auth/me')['data']['user'] ?? [];
    }

    public function logout(): void
    {
        $refreshToken = $this->client->getTokens()['refreshToken'] ?? null;
        if (! $refreshToken) {
            $this->client->clearTokens();
            return;
        }
        try {
            $this->client->request('POST', '/auth/logout', [], ['refreshToken' => $refreshToken], retry: false);
        } finally {
            $this->client->clearTokens();
        }
    }
}

final class ProductsResource extends Resource
{
    public function list(array $query = []): array { return $this->page('/products', 'products', $query); }
    public function get(string $uuid): array { return $this->client->request('GET', '/products/'.$this->id($uuid))['data']['product'] ?? []; }
    public function create(array $payload): array { return $this->client->request('POST', '/products', [], $payload)['data']['product'] ?? []; }
    public function update(string $uuid, array $payload): array { return $this->client->request('PUT', '/products/'.$this->id($uuid), [], $payload)['data']['product'] ?? []; }
    public function patch(string $uuid, array $payload): array { return $this->client->request('PATCH', '/products/'.$this->id($uuid), [], $payload)['data']['product'] ?? []; }
    public function updateStock(string $uuid, array $payload): array { return $this->client->request('PATCH', '/products/'.$this->id($uuid).'/stock', [], $payload)['data']['product'] ?? []; }
    public function updateVisibility(string $uuid, bool $isVisible): array { return $this->client->request('PATCH', '/products/'.$this->id($uuid).'/visibility', [], ['isVisible' => $isVisible])['data']['product'] ?? []; }
    public function delete(string $uuid): void { $this->client->request('DELETE', '/products/'.$this->id($uuid)); }
    private function id(string $uuid): string { return rawurlencode($uuid); }
}

final class OrdersResource extends Resource
{
    public function list(array $query = []): array { return $this->page('/orders', 'orders', $query); }
    public function get(string $uuid): array { return $this->client->request('GET', '/orders/'.rawurlencode($uuid))['data']['order'] ?? []; }
    public function create(array $payload): array { return $this->client->request('POST', '/orders', [], $payload)['data']['order'] ?? []; }
    public function updateStatus(string $uuid, string $status): array { return $this->client->request('PATCH', '/orders/'.rawurlencode($uuid).'/status', [], ['status' => $status])['data']['order'] ?? []; }
    public function updateTracking(string $uuid, array $payload): array { return $this->client->request('PATCH', '/orders/'.rawurlencode($uuid).'/tracking', [], $payload)['data']['order'] ?? []; }
    public function updatePaymentStatus(string $uuid, string $status): array { return $this->client->request('PATCH', '/orders/'.rawurlencode($uuid).'/payment-status', [], ['paymentStatus' => $status])['data']['order'] ?? []; }
    public function cancel(string $uuid): array { return $this->client->request('DELETE', '/orders/'.rawurlencode($uuid))['data']['order'] ?? []; }
}

final class CustomersResource extends Resource
{
    public function list(array $query = []): array { return $this->page('/customers', 'customers', $query); }
    public function get(string $uuid): array { return $this->client->request('GET', '/customers/'.rawurlencode($uuid))['data']['customer'] ?? []; }
}

final class StatsResource extends Resource
{
    public function overview(): array { return $this->client->request('GET', '/stats/overview')['data']; }
}

final class NotificationsResource extends Resource
{
    public function listEndpoints(): array { return $this->client->request('GET', '/notifications/endpoints')['data']['endpoints'] ?? []; }
    public function registerEndpoint(array $payload): array { return $this->client->request('POST', '/notifications/endpoints', [], $payload)['data']['endpoint'] ?? []; }
    public function removeEndpoint(array $payload): void { $this->client->request('DELETE', '/notifications/endpoints', [], $payload); }
}

final class MadabinaClient
{
    public readonly AuthResource $auth;
    public readonly ProductsResource $products;
    public readonly OrdersResource $orders;
    public readonly CustomersResource $customers;
    public readonly StatsResource $stats;
    public readonly NotificationsResource $notifications;
    private string $baseUrl;
    private ?TransportInterface $transport;
    private ?string $accessToken;
    private ?string $refreshToken;

    public function __construct(
        string $baseUrl = 'https://madabina.com/api/v1',
        ?string $accessToken = null,
        ?string $refreshToken = null,
        ?TransportInterface $transport = null,
    ) {
        $this->baseUrl = rtrim($baseUrl, '/');
        $this->accessToken = $accessToken;
        $this->refreshToken = $refreshToken;
        $this->transport = $transport ?? new NativeHttpTransport();
        $this->auth = new AuthResource($this);
        $this->products = new ProductsResource($this);
        $this->orders = new OrdersResource($this);
        $this->customers = new CustomersResource($this);
        $this->stats = new StatsResource($this);
        $this->notifications = new NotificationsResource($this);
    }

    /** @return array<string, string|null> */
    public function getTokens(): array
    {
        return array_filter(['accessToken' => $this->accessToken, 'refreshToken' => $this->refreshToken], static fn ($token) => $token !== null);
    }

    public function setTokens(array $tokens): void
    {
        $this->accessToken = $tokens['accessToken'] ?? null;
        $this->refreshToken = $tokens['refreshToken'] ?? null;
    }

    public function clearTokens(): void
    {
        $this->accessToken = null;
        $this->refreshToken = null;
    }

    public function getBaseUrl(): string
    {
        return $this->baseUrl;
    }

    /** @return array{data: array, meta: array} */
    public function request(string $method, string $path, array $query = [], ?array $body = null, bool $retry = true): array
    {
        $url = $this->baseUrl.$path;
        $query = array_filter($query, static fn ($value) => $value !== null);
        if ($query !== []) {
            $url .= '?'.http_build_query($query);
        }
        $headers = ['Accept' => 'application/json'];
        $encodedBody = null;
        if ($body !== null) {
            $headers['Content-Type'] = 'application/json';
            try {
                $encodedBody = json_encode($body, JSON_THROW_ON_ERROR | JSON_UNESCAPED_UNICODE);
            } catch (JsonException $error) {
                throw new RuntimeException('Could not encode the Madabina API request.', previous: $error);
            }
        }
        if ($this->accessToken !== null) {
            $headers['Authorization'] = 'Bearer '.$this->accessToken;
        }
        $response = $this->transport->request(strtoupper($method), $url, $headers, $encodedBody);
        $isAuthEndpoint = in_array($path, ['/auth/login', '/auth/refresh', '/auth/logout'], true);
        if ($response->status === 401 && $retry && ! $isAuthEndpoint && $this->refreshToken !== null) {
            $this->auth->refresh();
            return $this->request($method, $path, $query, $body, retry: false);
        }

        $decoded = null;
        if ($response->status !== 204 && $response->body !== '') {
            $decoded = json_decode($response->body, true);
        }
        if ($response->status < 200 || $response->status >= 300) {
            throw new MadabinaApiException(
                $decoded['message'] ?? "Madabina API request failed ({$response->status}).",
                $response->status,
                $decoded['details'] ?? null,
            );
        }
        if (is_array($decoded) && array_key_exists('success', $decoded)) {
            if (! $decoded['success']) {
                throw new MadabinaApiException($decoded['message'] ?? 'Madabina API request failed.', $response->status, $decoded['details'] ?? null);
            }
            return ['data' => $decoded['data'] ?? [], 'meta' => $decoded['meta'] ?? []];
        }

        return ['data' => is_array($decoded) ? $decoded : [], 'meta' => []];
    }
}
