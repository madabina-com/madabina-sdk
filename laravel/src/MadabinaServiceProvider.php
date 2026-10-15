<?php

declare(strict_types=1);

namespace Madabina\Laravel;

use Illuminate\Http\Client\Factory;
use Illuminate\Support\ServiceProvider;
use Madabina\Sdk\MadabinaClient;

final class MadabinaServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__.'/../config/madabina.php', 'madabina');

        $this->app->singleton(LaravelTransport::class, fn ($app) => new LaravelTransport(
            $app->make(Factory::class),
            (int) $app['config']->get('madabina.timeout', 30),
        ));

        $this->app->scoped(MadabinaClient::class, fn ($app) => new MadabinaClient(
            baseUrl: (string) $app['config']->get('madabina.base_url'),
            accessToken: $app['config']->get('madabina.access_token'),
            refreshToken: $app['config']->get('madabina.refresh_token'),
            transport: $app->make(LaravelTransport::class),
        ));
        $this->app->alias(MadabinaClient::class, 'madabina');
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->publishes([
                __DIR__.'/../config/madabina.php' => config_path('madabina.php'),
            ], 'madabina-config');
        }
    }
}
