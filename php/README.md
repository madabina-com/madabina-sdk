# madabina/sdk

Standalone PHP client for the Madabina merchant REST API (PHP 8.2+).

## Install from a local checkout

Until the package is published, add the SDK path repository to your application's `composer.json`. For the Madabina monorepo layout, where the application and `sdk/` directory are siblings, use:

```json
{
  "repositories": [
    {
      "type": "path",
      "url": "../sdk/php",
      "options": { "symlink": true, "versions": { "madabina/sdk": "0.1.0" } }
    }
  ],
  "require": {
    "madabina/sdk": "^0.1"
  }
}
```

Then run:

```sh
composer update madabina/sdk
```

Adjust `url` if your application is located elsewhere. Composer symlinks the local package, so edits in this checkout are immediately available to the application.

```php
use Madabina\Sdk\MadabinaClient;

$api = new MadabinaClient();
$session = $api->auth->login(['email' => getenv('MADABINA_EMAIL'), 'password' => getenv('MADABINA_PASSWORD')]);

try {
    $products = $api->products->list(['search' => 'phone', 'perPage' => 20]);
} finally {
    $api->auth->logout();
}
```

Pass `baseUrl`, `accessToken`, `refreshToken`, and a custom `TransportInterface` to the constructor when needed. Tokens stay in memory; persist them only through an application-owned secure store. The client retries one request after a 401 when a refresh token is available.

Resources include `auth`, `products`, `orders`, `customers`, `stats`, and `notifications`. List methods return `items`, `page`, `perPage`, `total`, and `lastPage`. API failures throw `MadabinaApiException`, with HTTP status and validation details.
