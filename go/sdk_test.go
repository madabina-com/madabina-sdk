package madabinasdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestProductsListRefreshesOnceAndNormalizesPagination(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.RequestURI()+" "+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch len(calls) {
		case 1:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"message":"Expired"}`))
		case 2:
			_, _ = w.Write([]byte(`{"success":true,"data":{"accessToken":"new-access","refreshToken":"new-refresh"}}`))
		default:
			_, _ = w.Write([]byte(`{"success":true,"data":{"products":[{"uuid":"p-1"}]},"meta":{"page":2,"perPage":10,"total":11,"lastPage":2}}`))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", WithHTTPClient(server.Client()), WithTokens("old-access", "old-refresh"))
	page, err := client.Products.List(context.Background(), Query{"search": "blue phone", "isVisible": true})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 11 || page.Page != 2 || len(page.Items) != 1 || page.Items[0].UUID != "p-1" {
		t.Fatalf("unexpected page: %#v", page)
	}
	if len(calls) != 3 || !strings.Contains(calls[1], "/auth/refresh") || !strings.HasSuffix(calls[2], "Bearer new-access") {
		t.Fatalf("unexpected request sequence: %#v", calls)
	}
	if got := client.Tokens(); !reflect.DeepEqual(got, TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}) {
		t.Fatalf("tokens not refreshed: %#v", got)
	}
}

func TestAPIErrorIncludesValidationDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"success":false,"message":"Invalid data","statusCode":422,"details":{"email":["Required"]}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", WithHTTPClient(server.Client()))
	_, err := client.Auth.Login(context.Background(), LoginCredentials{Email: "", Password: ""})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != 422 {
		t.Fatalf("expected 422 APIError, got %#v", err)
	}
	var details map[string][]string
	encoded, _ := json.Marshal(apiErr.Details)
	if err := json.Unmarshal(encoded, &details); err != nil || !reflect.DeepEqual(details, map[string][]string{"email": {"Required"}}) {
		t.Fatalf("unexpected details: %#v (%v)", apiErr.Details, err)
	}
}

func TestDeleteAcceptsNoContent(t *testing.T) {
	var requestURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.URL.RequestURI()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", WithHTTPClient(server.Client()), WithTokens("token", ""))
	if err := client.Products.Delete(context.Background(), "product/1"); err != nil {
		t.Fatal(err)
	}
	if requestURI != "/api/v1/products/product%2F1" {
		t.Fatalf("unexpected URL path: %s", requestURI)
	}
}

func TestExposesBothPutAndPatchProductUpdateRoutes(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		_, _ = w.Write([]byte(`{"success":true,"data":{"product":{"uuid":"p-1"}}}`))
	}))
	defer server.Close()
	client := NewClient(server.URL+"/api/v1", WithHTTPClient(server.Client()), WithTokens("token", ""))
	if _, err := client.Products.Update(context.Background(), "p-1", ProductInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Products.Patch(context.Background(), "p-1", ProductInput{TitleFr: "Nouveau"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(methods, []string{"PUT", "PATCH"}) {
		t.Fatalf("unexpected methods: %#v", methods)
	}
}

func TestProvidesCallableForEveryDocumentedRESTV1Operation(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+strings.TrimPrefix(r.URL.Path, "/api/v1"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"accessToken":"access","refreshToken":"refresh","user":{"uuid":"u-1"},"products":[],"product":{"uuid":"p-1"},"orders":[],"order":{"uuid":"o-1"},"customers":[],"customer":{"uuid":"c-1"},"endpoints":[],"endpoint":{"id":1}},"meta":{"page":1,"perPage":15,"total":0,"lastPage":1}}`))
	}))
	defer server.Close()
	client := NewClient(server.URL+"/api/v1", WithHTTPClient(server.Client()))
	ctx := context.Background()
	if _, err := client.Auth.Login(ctx, LoginCredentials{Email: "a@example.com", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Auth.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Auth.Me(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Auth.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Stats.Overview(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Notifications.ListEndpoints(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Notifications.RegisterEndpoint(ctx, NotificationEndpointInput{Channel: "web_push", Endpoint: "endpoint"}); err != nil {
		t.Fatal(err)
	}
	if err := client.Notifications.RemoveEndpoint(ctx, NotificationEndpointInput{Channel: "web_push", Endpoint: "endpoint"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Products.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Products.Create(ctx, ProductInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Products.Get(ctx, "p-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Products.Update(ctx, "p-1", ProductInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Products.Patch(ctx, "p-1", ProductInput{}); err != nil {
		t.Fatal(err)
	}
	delta := 1
	if _, err := client.Products.UpdateStock(ctx, "p-1", StockInput{Delta: &delta}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Products.UpdateVisibility(ctx, "p-1", true); err != nil {
		t.Fatal(err)
	}
	if err := client.Products.Delete(ctx, "p-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders.Create(ctx, OrderInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders.Get(ctx, "o-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders.Cancel(ctx, "o-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders.UpdateStatus(ctx, "o-1", OrderShipped); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders.UpdateTracking(ctx, "o-1", "Carrier", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders.UpdatePaymentStatus(ctx, "o-1", "PAID"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Customers.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Customers.Get(ctx, "c-1"); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"POST /auth/login", "POST /auth/refresh", "GET /auth/me", "POST /auth/logout",
		"GET /stats/overview", "GET /notifications/endpoints", "POST /notifications/endpoints", "DELETE /notifications/endpoints",
		"GET /products", "POST /products", "GET /products/p-1", "PUT /products/p-1", "PATCH /products/p-1",
		"PATCH /products/p-1/stock", "PATCH /products/p-1/visibility", "DELETE /products/p-1",
		"GET /orders", "POST /orders", "GET /orders/o-1", "DELETE /orders/o-1", "PATCH /orders/o-1/status",
		"PATCH /orders/o-1/tracking", "PATCH /orders/o-1/payment-status", "GET /customers", "GET /customers/c-1",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected route coverage:\n got %#v\nwant %#v", calls, want)
	}
}

func TestConcurrentExpiredRequestsShareOneRefresh(t *testing.T) {
	var oldRequests atomic.Int32
	var refreshRequests atomic.Int32
	var release sync.Once
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/auth/refresh" {
			refreshRequests.Add(1)
			_, _ = w.Write([]byte(`{"success":true,"data":{"accessToken":"new-access","refreshToken":"new-refresh"}}`))
			return
		}
		if r.Header.Get("Authorization") == "Bearer old-access" {
			if oldRequests.Add(1) == 2 {
				release.Do(func() { close(ready) })
			}
			<-ready
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"message":"Expired"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"product":{"uuid":"p-1"}}}`))
	}))
	defer server.Close()
	client := NewClient(server.URL+"/api/v1", WithHTTPClient(server.Client()), WithTokens("old-access", "old-refresh"))

	var workers sync.WaitGroup
	for _, uuid := range []string{"p-1", "p-2"} {
		workers.Add(1)
		go func(id string) {
			defer workers.Done()
			if _, err := client.Products.Get(context.Background(), id); err != nil {
				t.Error(err)
			}
		}(uuid)
	}
	workers.Wait()
	if refreshRequests.Load() != 1 || oldRequests.Load() != 2 {
		t.Fatalf("unexpected refresh/request counts: refresh=%d old=%d", refreshRequests.Load(), oldRequests.Load())
	}
}

func TestRejectedRefreshClearsCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"message":"Invalid refresh token"}`))
	}))
	defer server.Close()
	client := NewClient(server.URL+"/api/v1", WithHTTPClient(server.Client()), WithTokens("expired-access", "revoked-refresh"))
	_, err := client.Auth.Refresh(context.Background())
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("expected refresh APIError, got %#v", err)
	}
	if tokens := client.Tokens(); tokens != (TokenPair{}) {
		t.Fatalf("credentials were not cleared: %#v", tokens)
	}
}
