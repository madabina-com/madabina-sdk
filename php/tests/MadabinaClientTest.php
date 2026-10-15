<?php

use Madabina\Sdk\HttpResponse;
use Madabina\Sdk\MadabinaApiException;
use Madabina\Sdk\MadabinaClient;
use Madabina\Sdk\TransportInterface;
use PHPUnit\Framework\TestCase;

final class MadabinaClientTest extends TestCase
{
    public function test_refreshes_and_retries_a_paginated_request(): void
    {
        $transport = new QueueTransport([
            new HttpResponse(401, '{"success":false,"message":"Expired"}'),
            new HttpResponse(200, '{"success":true,"data":{"accessToken":"new-access","refreshToken":"new-refresh"}}'),
            new HttpResponse(200, '{"success":true,"data":{"products":[{"uuid":"p-1"}]},"meta":{"page":2,"perPage":10,"total":11,"lastPage":2}}'),
        ]);
        $client = new MadabinaClient('https://example.test/api/v1/', 'old-access', 'old-refresh', $transport);

        self::assertSame([
            'items' => [['uuid' => 'p-1']], 'page' => 2, 'perPage' => 10, 'total' => 11, 'lastPage' => 2,
        ], $client->products->list(['search' => 'blue phone', 'isVisible' => true]));
        self::assertCount(3, $transport->requests);
        self::assertSame('Bearer old-access', $transport->requests[0]['headers']['Authorization']);
        self::assertStringContainsString('/auth/refresh', $transport->requests[1]['url']);
        self::assertSame('Bearer new-access', $transport->requests[2]['headers']['Authorization']);
        self::assertStringContainsString('search=blue+phone', $transport->requests[2]['url']);
        self::assertSame(['accessToken' => 'new-access', 'refreshToken' => 'new-refresh'], $client->getTokens());
    }

    public function test_exposes_validation_details_in_api_exception(): void
    {
        $client = new MadabinaClient(transport: new QueueTransport([
            new HttpResponse(422, '{"success":false,"message":"Invalid data","statusCode":422,"details":{"email":["Required"]}}'),
        ]));

        try {
            $client->auth->login(['email' => '', 'password' => '']);
            self::fail('The request should have thrown.');
        } catch (MadabinaApiException $exception) {
            self::assertSame(422, $exception->status);
            self::assertSame(['email' => ['Required']], $exception->details);
        }
    }

    public function test_delete_204_returns_without_json_decoding(): void
    {
        $transport = new QueueTransport([
            new HttpResponse(204, ''),
        ]);
        $client = new MadabinaClient(accessToken: 'token', transport: $transport);

        $client->products->delete('product/1');
        self::assertSame('https://madabina.com/api/v1/products/product%2F1', $transport->requests[0]['url']);
    }

    public function test_exposes_both_put_and_patch_product_update_routes(): void
    {
        $transport = new QueueTransport([
            new HttpResponse(200, '{"success":true,"data":{"product":{"uuid":"p-1"}}}'),
            new HttpResponse(200, '{"success":true,"data":{"product":{"uuid":"p-1"}}}'),
        ]);
        $client = new MadabinaClient(accessToken: 'token', transport: $transport);
        $client->products->update('p-1', ['titleFr' => 'Nouveau']);
        $client->products->patch('p-1', ['titleFr' => 'Nouveau']);

        self::assertSame(['PUT', 'PATCH'], array_column($transport->requests, 'method'));
    }

    public function test_provides_a_callable_for_every_documented_rest_v1_operation(): void
    {
        $body = '{"success":true,"data":{"accessToken":"access","refreshToken":"refresh","user":{"uuid":"u-1"},"products":[],"product":{"uuid":"p-1"},"orders":[],"order":{"uuid":"o-1"},"customers":[],"customer":{"uuid":"c-1"},"endpoints":[],"endpoint":{"id":1}},"meta":{"page":1,"perPage":15,"total":0,"lastPage":1}}';
        $transport = new QueueTransport(array_fill(0, 25, new HttpResponse(200, $body)));
        $client = new MadabinaClient('https://example.test/api/v1', transport: $transport);

        $client->auth->login(['email' => 'a@example.com', 'password' => 'secret']);
        $client->auth->refresh();
        $client->auth->me();
        $client->auth->logout();
        $client->stats->overview();
        $client->notifications->listEndpoints();
        $client->notifications->registerEndpoint(['channel' => 'web_push', 'endpoint' => 'endpoint']);
        $client->notifications->removeEndpoint(['channel' => 'web_push', 'endpoint' => 'endpoint']);
        $client->products->list();
        $client->products->create(['titleFr' => 'x']);
        $client->products->get('p-1');
        $client->products->update('p-1', ['titleFr' => 'x']);
        $client->products->patch('p-1', ['titleFr' => 'x']);
        $client->products->updateStock('p-1', ['delta' => 1]);
        $client->products->updateVisibility('p-1', true);
        $client->products->delete('p-1');
        $client->orders->list();
        $client->orders->create(['customer' => [], 'items' => []]);
        $client->orders->get('o-1');
        $client->orders->cancel('o-1');
        $client->orders->updateStatus('o-1', 'SHIPPED');
        $client->orders->updateTracking('o-1', ['carrier' => 'Carrier']);
        $client->orders->updatePaymentStatus('o-1', 'PAID');
        $client->customers->list();
        $client->customers->get('c-1');

        $calls = array_map(static fn (array $request): string => $request['method'].' '.str_replace('/api/v1', '', parse_url($request['url'], PHP_URL_PATH)), $transport->requests);
        self::assertSame([
            'POST /auth/login', 'POST /auth/refresh', 'GET /auth/me', 'POST /auth/logout',
            'GET /stats/overview', 'GET /notifications/endpoints', 'POST /notifications/endpoints', 'DELETE /notifications/endpoints',
            'GET /products', 'POST /products', 'GET /products/p-1', 'PUT /products/p-1', 'PATCH /products/p-1',
            'PATCH /products/p-1/stock', 'PATCH /products/p-1/visibility', 'DELETE /products/p-1',
            'GET /orders', 'POST /orders', 'GET /orders/o-1', 'DELETE /orders/o-1', 'PATCH /orders/o-1/status',
            'PATCH /orders/o-1/tracking', 'PATCH /orders/o-1/payment-status', 'GET /customers', 'GET /customers/c-1',
        ], $calls);
    }

    public function test_clears_credentials_when_refresh_is_rejected(): void
    {
        $client = new MadabinaClient(accessToken: 'expired-access', refreshToken: 'revoked-refresh', transport: new QueueTransport([
            new HttpResponse(401, '{"success":false,"message":"Invalid refresh token"}'),
        ]));

        try {
            $client->auth->refresh();
            self::fail('The refresh request should have thrown.');
        } catch (MadabinaApiException $exception) {
            self::assertSame(401, $exception->status);
            self::assertSame([], $client->getTokens());
        }
    }
}

final class QueueTransport implements TransportInterface
{
    public array $requests = [];

    /** @param list<HttpResponse> $responses */
    public function __construct(private array $responses)
    {
    }

    public function request(string $method, string $url, array $headers, ?string $body): HttpResponse
    {
        $this->requests[] = compact('method', 'url', 'headers', 'body');

        return array_shift($this->responses) ?? throw new RuntimeException('No queued response.');
    }
}
