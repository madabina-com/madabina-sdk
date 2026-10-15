<?php

use Illuminate\Foundation\Application;
use Illuminate\Config\Repository;
use Illuminate\Http\Client\Factory;
use Illuminate\Http\Client\Request;
use Madabina\Laravel\Facades\Madabina;
use Madabina\Laravel\LaravelTransport;
use Madabina\Laravel\MadabinaServiceProvider;
use Madabina\Sdk\MadabinaClient;
use PHPUnit\Framework\TestCase;

final class MadabinaServiceProviderTest extends TestCase
{
    protected function tearDown(): void
    {
        Madabina::clearResolvedInstances();
        Madabina::setFacadeApplication(null);
        parent::tearDown();
    }

    public function test_provider_injects_configured_sdk_and_facade_resolves_same_instance(): void
    {
        $app = new Application(dirname(__DIR__, 3).'/web-app');
        $app->instance('config', new Repository(['madabina' => [
            'base_url' => 'https://staging.example/api/v1',
            'access_token' => 'access-token',
            'refresh_token' => 'refresh-token',
            'timeout' => 9,
        ]]));
        $app->instance(Factory::class, new Factory());

        $provider = new MadabinaServiceProvider($app);
        $provider->register();
        Madabina::setFacadeApplication($app);

        $client = $app->make(MadabinaClient::class);
        self::assertSame(['accessToken' => 'access-token', 'refreshToken' => 'refresh-token'], $client->getTokens());
        self::assertSame($client, Madabina::client());
        self::assertSame('https://staging.example/api/v1', $client->getBaseUrl());
    }

    public function test_laravel_transport_preserves_method_headers_and_json_body(): void
    {
        $http = new Factory();
        $http->fake(fn () => Factory::response('{"success":true,"data":{}}', 201));
        $transport = new LaravelTransport($http, 9);

        $response = $transport->request('POST', 'https://example.test/api/v1/products', [
            'Accept' => 'application/json',
            'Authorization' => 'Bearer access',
        ], '{"titleFr":"Phone"}');

        self::assertSame(201, $response->status);
        self::assertSame('{"success":true,"data":{}}', $response->body);
        $http->assertSent(fn (Request $request): bool => $request->method() === 'POST'
            && $request->url() === 'https://example.test/api/v1/products'
            && $request->hasHeader('Authorization', 'Bearer access')
            && $request->data() === ['titleFr' => 'Phone']);
    }
}
