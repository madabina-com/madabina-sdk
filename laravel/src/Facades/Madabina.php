<?php

declare(strict_types=1);

namespace Madabina\Laravel\Facades;

use Illuminate\Support\Facades\Facade;
use Madabina\Sdk\MadabinaClient;

/** @method static MadabinaClient client() */
final class Madabina extends Facade
{
    public static function client(): MadabinaClient
    {
        return static::getFacadeRoot();
    }

    protected static function getFacadeAccessor(): string
    {
        return MadabinaClient::class;
    }
}
