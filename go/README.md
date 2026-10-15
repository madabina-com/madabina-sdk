# Madabina Go SDK

Go client for the Madabina merchant REST API (Go 1.27.2+).

```go
import (
	"context"
	"os"

	madabinasdk "github.com/madabina-com/sdk-go"
)

func listProducts(ctx context.Context) error {
	client := madabinasdk.NewClient("https://madabina.com/api/v1")
	_, err := client.Auth.Login(ctx, madabinasdk.LoginCredentials{
		Email: os.Getenv("MADABINA_EMAIL"), Password: os.Getenv("MADABINA_PASSWORD"),
	})
	if err != nil {
		return err
	}
	defer client.Auth.Logout(ctx)

	products, err := client.Products.List(ctx, madabinasdk.Query{"search": "phone"})
	_ = products
	return err
}
```

`WithHTTPClient` lets applications provide an `http.Client` or another `Do`-compatible transport. `WithTokens` accepts an existing token pair. Tokens are held in memory and the client refreshes once after a 401 when a refresh token is set.

All operations accept `context.Context`; list methods return typed pages with pagination metadata. API failures use `*madabinasdk.APIError`, which includes the HTTP status and validation details.
