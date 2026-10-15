# Madabina REST SDKs

This repository contains idiomatic clients for the Madabina merchant REST API:

| Package | Runtime | Directory |
| --- | --- | --- |
| [`@madabina/sdk`](javascript/README.md) | JavaScript and TypeScript, Node.js 22.12+ | `javascript/` |
| [`madabina/sdk`](php/README.md) | PHP 8.2+ | `php/` |
| [`madabina/laravel-sdk`](laravel/README.md) | Laravel 12+ | `laravel/` |
| [`github.com/madabina-com/sdk-go`](go/README.md) | Go 1.27.2+ | `go/` |

Each client covers authentication and token refresh, products, orders, customers, overview statistics, and push notification endpoints. Tokens are held in memory and remain under the consuming application's control.

## API contract

[`openapi.json`](openapi.json) is the canonical OpenAPI 3.0.3 contract. The Laravel application serves its byte-identical copy from `/api/openapi.json`; rebuild or check both copies with:

```sh
python scripts/build_openapi.py
python scripts/build_openapi.py --check
```

## Verification

```sh
npm test
node test/contract.test.mjs
phpunit --configuration php/phpunit.xml
phpunit --configuration laravel/phpunit.xml
cd go && go test ./...
```

The PHP and Laravel suites use PHPUnit 11.5. Package publication is intentionally separate from this repository's local build and verification.
