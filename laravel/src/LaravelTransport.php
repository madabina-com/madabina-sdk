<?php

declare(strict_types=1);

namespace Madabina\Laravel;

use Illuminate\Http\Client\Factory;
use Madabina\Sdk\HttpResponse;
use Madabina\Sdk\TransportInterface;

final class LaravelTransport implements TransportInterface
{
    public function __construct(private readonly Factory $http, private readonly int $timeout = 30)
    {
    }

    public function request(string $method, string $url, array $headers, ?string $body): HttpResponse
    {
        $request = $this->http
            ->acceptJson()
            ->withHeaders($headers)
            ->timeout($this->timeout)
            ->withOptions(['http_errors' => false]);

        if ($body !== null) {
            $request->withBody($body, 'application/json');
        }

        $response = $request->send($method, $url);

        return new HttpResponse($response->status(), $response->body());
    }
}
