# @madabina/sdk

Typed JavaScript/TypeScript client for the Madabina merchant REST API.

```js
import { MadabinaClient } from '@madabina/sdk';

const api = new MadabinaClient();
const { user } = await api.auth.login({
  email: process.env.MADABINA_EMAIL,
  password: process.env.MADABINA_PASSWORD,
});

try {
  const products = await api.products.list({ search: 'phone', perPage: 20 });
  console.log(user.email, products.items.length);
} finally {
  await api.auth.logout();
}
```

Tokens are held in memory. Supply existing tokens with `accessToken` and `refreshToken`, or use `setTokens`; persist them in the consuming application if needed. The client refreshes once and retries an API request after a 401 when a refresh token is available.

The client uses the platform Fetch API and supports a custom `fetch` implementation for tests and runtimes. Configure `baseUrl` for local or staging APIs.
