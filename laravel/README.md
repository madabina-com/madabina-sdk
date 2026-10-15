# madabina/laravel-sdk

Laravel 12 integration for the standalone [`madabina/sdk`](../php) package. Laravel discovers the service provider automatically.

## Install from a local checkout

Until the packages are published, add both local path repositories to the Laravel application's `composer.json`. For the Madabina monorepo layout, where `web-app/` and `sdk/` are siblings, use:

```json
{
  "repositories": [
    {
      "type": "path",
      "url": "../sdk/php",
      "options": { "symlink": true, "versions": { "madabina/sdk": "0.1.0" } }
    },
    {
      "type": "path",
      "url": "../sdk/laravel",
      "options": { "symlink": true, "versions": { "madabina/laravel-sdk": "0.1.0" } }
    }
  ],
  "require": {
    "madabina/sdk": "^0.1",
    "madabina/laravel-sdk": "^0.1"
  }
}
```

Then run:

```sh
composer update madabina/sdk madabina/laravel-sdk
```

Adjust the `url` values if your Laravel application is located elsewhere. Composer symlinks the local packages, so edits in this checkout are immediately available to the application.

After publication to Packagist, the installation command becomes:

```sh
composer require madabina/sdk madabina/laravel-sdk
php artisan vendor:publish --tag=madabina-config
```

The package registers `Madabina\Sdk\MadabinaClient` in the service container and provides the `Madabina` facade:

```php
use Madabina\Laravel\Facades\Madabina;

$products = Madabina::client()->products->list(['search' => 'phone']);
```

Set `MADABINA_API_URL` to use a local or staging API. Tokens are not written to disk by this package. Set tokens on the resolved client from an application-owned secure session or secret store:

```php
$client = app(\Madabina\Sdk\MadabinaClient::class);
$client->setTokens($sessionTokens);
```
