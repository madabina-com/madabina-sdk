package madabinasdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

const defaultBaseURL = "https://madabina.com/api/v1"

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Option func(*Client)

func WithHTTPClient(client HTTPDoer) Option { return func(c *Client) { c.httpClient = client } }
func WithTokens(accessToken, refreshToken string) Option {
	return func(c *Client) { c.SetTokens(TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}) }
}

type Client struct {
	baseURL    string
	httpClient HTTPDoer
	tokenMu    sync.RWMutex
	tokens     TokenPair
	refreshMu  sync.Mutex

	Auth          *AuthResource
	Products      *ProductsResource
	Orders        *OrdersResource
	Customers     *CustomersResource
	Stats         *StatsResource
	Notifications *NotificationsResource
}

func NewClient(baseURL string, options ...Option) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	c := &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: http.DefaultClient}
	for _, option := range options {
		option(c)
	}
	c.Auth = &AuthResource{client: c}
	c.Products = &ProductsResource{client: c}
	c.Orders = &OrdersResource{client: c}
	c.Customers = &CustomersResource{client: c}
	c.Stats = &StatsResource{client: c}
	c.Notifications = &NotificationsResource{client: c}
	return c
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
}

func (c *Client) Tokens() TokenPair {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.tokens
}

func (c *Client) SetTokens(tokens TokenPair) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.tokens = tokens
}

func (c *Client) ClearTokens() { c.SetTokens(TokenPair{}) }

type APIError struct {
	Status  int
	Message string
	Details any
}

func (e *APIError) Error() string { return e.Message }

type apiFailure struct {
	Message string `json:"message"`
	Details any    `json:"details"`
}

type apiEnvelope struct {
	Success *bool           `json:"success"`
	Data    json.RawMessage `json:"data"`
	Meta    PaginationMeta  `json:"meta"`
	Message string          `json:"message"`
	Details any             `json:"details"`
}

func apiRequest[T any](c *Client, ctx context.Context, method, path string, query Query, body any, retry bool) (T, PaginationMeta, error) {
	var zero T
	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return zero, PaginationMeta{}, err
	}
	values := endpoint.Query()
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if query[key] != nil {
			values.Set(key, fmt.Sprint(query[key]))
		}
	}
	endpoint.RawQuery = values.Encode()
	var encodedBody []byte
	if body != nil {
		encodedBody, err = json.Marshal(body)
		if err != nil {
			return zero, PaginationMeta{}, fmt.Errorf("encode request: %w", err)
		}
	}
	usedToken := c.Tokens().AccessToken
	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(encodedBody)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return zero, PaginationMeta{}, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if usedToken != "" {
		request.Header.Set("Authorization", "Bearer "+usedToken)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return zero, PaginationMeta{}, err
	}
	defer response.Body.Close()

	pathOnly := path
	isAuthEndpoint := pathOnly == "/auth/login" || pathOnly == "/auth/refresh" || pathOnly == "/auth/logout"
	if response.StatusCode == http.StatusUnauthorized && retry && !isAuthEndpoint && c.Tokens().RefreshToken != "" {
		_, _ = io.Copy(io.Discard, response.Body)
		if err := c.refreshAfterUnauthorized(ctx, usedToken); err != nil {
			return zero, PaginationMeta{}, err
		}
		return apiRequest[T](c, ctx, method, path, query, body, false)
	}
	if response.StatusCode == http.StatusNoContent {
		return zero, PaginationMeta{}, nil
	}
	var envelope apiEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return zero, PaginationMeta{}, &APIError{Status: response.StatusCode, Message: fmt.Sprintf("Madabina API request failed (%d).", response.StatusCode)}
		}
		return zero, PaginationMeta{}, fmt.Errorf("decode Madabina API response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || (envelope.Success != nil && !*envelope.Success) {
		message := envelope.Message
		if message == "" {
			message = fmt.Sprintf("Madabina API request failed (%d).", response.StatusCode)
		}
		return zero, PaginationMeta{}, &APIError{Status: response.StatusCode, Message: message, Details: envelope.Details}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return zero, envelope.Meta, nil
	}
	if err := json.Unmarshal(envelope.Data, &zero); err != nil {
		return zero, PaginationMeta{}, fmt.Errorf("decode Madabina API data: %w", err)
	}
	return zero, envelope.Meta, nil
}

func (c *Client) refreshAfterUnauthorized(ctx context.Context, usedToken string) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	tokens := c.Tokens()
	if tokens.AccessToken != "" && tokens.AccessToken != usedToken {
		return nil
	}
	if tokens.RefreshToken == "" {
		return &APIError{Status: http.StatusUnauthorized, Message: "No refresh token is available."}
	}
	_, err := c.Auth.Refresh(ctx)
	return err
}

type Query map[string]any

type PaginationMeta struct {
	Page     int `json:"page"`
	PerPage  int `json:"perPage"`
	Total    int `json:"total"`
	LastPage int `json:"lastPage"`
}

type Page[T any] struct {
	Items []T
	PaginationMeta
}

type LoginCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	UUID            string  `json:"uuid"`
	Email           string  `json:"email"`
	StoreName       *string `json:"storeName"`
	Phone           *string `json:"phone"`
	Currency        string  `json:"currency"`
	DefaultLanguage string  `json:"defaultLanguage"`
	Stores          []Store `json:"stores"`
}

type Store struct {
	UUID            string  `json:"uuid"`
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	Phone           *string `json:"phone"`
	Currency        string  `json:"currency"`
	DefaultLanguage string  `json:"defaultLanguage"`
}

type LoginResult struct {
	Tokens TokenPair `json:"tokens"`
	User   User      `json:"user"`
}

type AuthResource struct{ client *Client }

func (r *AuthResource) Login(ctx context.Context, credentials LoginCredentials) (LoginResult, error) {
	r.client.ClearTokens()
	data, _, err := apiRequest[struct {
		TokenPair
		User User `json:"user"`
	}](r.client, ctx, http.MethodPost, "/auth/login", nil, credentials, false)
	if err != nil {
		return LoginResult{}, err
	}
	r.client.SetTokens(data.TokenPair)
	return LoginResult{Tokens: data.TokenPair, User: data.User}, nil
}

func (r *AuthResource) Refresh(ctx context.Context) (TokenPair, error) {
	tokens := r.client.Tokens()
	if tokens.RefreshToken == "" {
		return TokenPair{}, &APIError{Status: http.StatusUnauthorized, Message: "No refresh token is available."}
	}
	updated, _, err := apiRequest[TokenPair](r.client, ctx, http.MethodPost, "/auth/refresh", nil, map[string]string{"refreshToken": tokens.RefreshToken}, false)
	if err != nil {
		r.client.ClearTokens()
		return TokenPair{}, err
	}
	r.client.SetTokens(updated)
	return updated, nil
}

func (r *AuthResource) Me(ctx context.Context) (User, error) {
	data, _, err := apiRequest[struct {
		User User `json:"user"`
	}](r.client, ctx, http.MethodGet, "/auth/me", nil, nil, true)
	return data.User, err
}

func (r *AuthResource) Logout(ctx context.Context) error {
	tokens := r.client.Tokens()
	if tokens.RefreshToken == "" {
		r.client.ClearTokens()
		return nil
	}
	_, _, err := apiRequest[map[string]any](r.client, ctx, http.MethodPost, "/auth/logout", nil, map[string]string{"refreshToken": tokens.RefreshToken}, false)
	r.client.ClearTokens()
	return err
}

type ProductVariantInput struct {
	UUID            string `json:"uuid,omitempty"`
	SKU             string `json:"sku"`
	Attribute       string `json:"attribute,omitempty"`
	Color           string `json:"color,omitempty"`
	Size            string `json:"size,omitempty"`
	CustomAttribute string `json:"customAttribute,omitempty"`
	Price           int    `json:"price"`
	CompareAtPrice  int    `json:"compareAtPrice,omitempty"`
	StockQuantity   int    `json:"stockQuantity"`
}

type ProductInput struct {
	StoreUUID        string                `json:"storeUuid,omitempty"`
	SKU              string                `json:"sku,omitempty"`
	TitleFr          string                `json:"titleFr"`
	TitleAr          string                `json:"titleAr"`
	TitleEn          string                `json:"titleEn"`
	DescriptionFr    string                `json:"descriptionFr,omitempty"`
	DescriptionAr    string                `json:"descriptionAr,omitempty"`
	DescriptionEn    string                `json:"descriptionEn,omitempty"`
	Category         ProductCategoryCode   `json:"category"`
	Price            int                   `json:"price"`
	CompareAtPrice   int                   `json:"compareAtPrice,omitempty"`
	ProductType      string                `json:"productType,omitempty"`
	CostPrice        int                   `json:"costPrice,omitempty"`
	StockQuantity    int                   `json:"stockQuantity"`
	MinStockAlert    int                   `json:"minStockAlert,omitempty"`
	IsVisibleOnStore *bool                 `json:"isVisibleOnStore,omitempty"`
	ImageURL         string                `json:"imageUrl,omitempty"`
	Variants         []ProductVariantInput `json:"variants,omitempty"`
}

type ProductCategoryCode string

const (
	CategoryElectronics ProductCategoryCode = "ELECTRONICS"
	CategoryFashion     ProductCategoryCode = "FASHION"
	CategoryBeauty      ProductCategoryCode = "BEAUTY"
	CategoryHome        ProductCategoryCode = "HOME"
	CategoryCrafts      ProductCategoryCode = "CRAFTS"
)

type StockInput struct {
	VariantUUID *string `json:"variantUuid,omitempty"`
	Delta       *int    `json:"delta,omitempty"`
	NewQuantity *int    `json:"newQuantity,omitempty"`
}

type Product struct {
	UUID             string            `json:"uuid"`
	StoreUUID        *string           `json:"storeUuid"`
	SKU              *string           `json:"sku"`
	TitleFr          string            `json:"titleFr"`
	TitleAr          *string           `json:"titleAr"`
	TitleEn          string            `json:"titleEn"`
	DescriptionFr    *string           `json:"descriptionFr"`
	DescriptionAr    *string           `json:"descriptionAr"`
	DescriptionEn    *string           `json:"descriptionEn"`
	Category         *string           `json:"category"`
	Categories       []ProductCategory `json:"categories"`
	Price            int               `json:"price"`
	ProductType      string            `json:"productType"`
	CompareAtPrice   *int              `json:"compareAtPrice"`
	CostPrice        *int              `json:"costPrice"`
	StockQuantity    int               `json:"stockQuantity"`
	MinStockAlert    *int              `json:"minStockAlert"`
	IsVisibleOnStore bool              `json:"isVisibleOnStore"`
	ImageURL         *string           `json:"imageUrl"`
	CreatedAt        *string           `json:"createdAt"`
	UpdatedAt        *string           `json:"updatedAt"`
	MarketPrices     []MarketPrice     `json:"marketPrices"`
	Variants         []ProductVariant  `json:"variants"`
}

type ProductCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type MarketPrice struct {
	MarketID       int     `json:"marketId"`
	CountryCode    *string `json:"countryCode"`
	CurrencyCode   *string `json:"currencyCode"`
	Price          int     `json:"price"`
	CompareAtPrice *int    `json:"compareAtPrice"`
	CostPrice      *int    `json:"costPrice,omitempty"`
}
type ProductVariant struct {
	UUID            string        `json:"uuid"`
	SKU             *string       `json:"sku"`
	Attribute       *string       `json:"attribute"`
	Color           *string       `json:"color"`
	Size            *string       `json:"size"`
	CustomAttribute *string       `json:"customAttribute"`
	MediaID         *int          `json:"mediaId"`
	Price           int           `json:"price"`
	CompareAtPrice  *int          `json:"compareAtPrice"`
	StockQuantity   int           `json:"stockQuantity"`
	IsActive        bool          `json:"isActive"`
	MarketPrices    []MarketPrice `json:"marketPrices"`
}

type ProductsResource struct{ client *Client }

func (r *ProductsResource) List(ctx context.Context, query Query) (Page[Product], error) {
	data, meta, err := apiRequest[struct {
		Products []Product `json:"products"`
	}](r.client, ctx, http.MethodGet, "/products", query, nil, true)
	return Page[Product]{Items: data.Products, PaginationMeta: meta}, err
}
func (r *ProductsResource) Get(ctx context.Context, uuid string) (Product, error) {
	data, _, err := apiRequest[struct {
		Product Product `json:"product"`
	}](r.client, ctx, http.MethodGet, "/products/"+url.PathEscape(uuid), nil, nil, true)
	return data.Product, err
}
func (r *ProductsResource) Create(ctx context.Context, input ProductInput) (Product, error) {
	data, _, err := apiRequest[struct {
		Product Product `json:"product"`
	}](r.client, ctx, http.MethodPost, "/products", nil, input, true)
	return data.Product, err
}
func (r *ProductsResource) Update(ctx context.Context, uuid string, input ProductInput) (Product, error) {
	data, _, err := apiRequest[struct {
		Product Product `json:"product"`
	}](r.client, ctx, http.MethodPut, "/products/"+url.PathEscape(uuid), nil, input, true)
	return data.Product, err
}
func (r *ProductsResource) Patch(ctx context.Context, uuid string, input ProductInput) (Product, error) {
	data, _, err := apiRequest[struct {
		Product Product `json:"product"`
	}](r.client, ctx, http.MethodPatch, "/products/"+url.PathEscape(uuid), nil, input, true)
	return data.Product, err
}
func (r *ProductsResource) UpdateStock(ctx context.Context, uuid string, input StockInput) (Product, error) {
	data, _, err := apiRequest[struct {
		Product Product `json:"product"`
	}](r.client, ctx, http.MethodPatch, "/products/"+url.PathEscape(uuid)+"/stock", nil, input, true)
	return data.Product, err
}
func (r *ProductsResource) UpdateVisibility(ctx context.Context, uuid string, visible bool) (Product, error) {
	data, _, err := apiRequest[struct {
		Product Product `json:"product"`
	}](r.client, ctx, http.MethodPatch, "/products/"+url.PathEscape(uuid)+"/visibility", nil, map[string]bool{"isVisible": visible}, true)
	return data.Product, err
}
func (r *ProductsResource) Delete(ctx context.Context, uuid string) error {
	_, _, err := apiRequest[map[string]any](r.client, ctx, http.MethodDelete, "/products/"+url.PathEscape(uuid), nil, nil, true)
	return err
}

type OrderStatus string

const (
	OrderPending    OrderStatus = "PENDING"
	OrderConfirmed  OrderStatus = "CONFIRMED"
	OrderProcessing OrderStatus = "PROCESSING"
	OrderShipped    OrderStatus = "SHIPPED"
	OrderDelivered  OrderStatus = "DELIVERED"
	OrderCancelled  OrderStatus = "CANCELLED"
	OrderReturned   OrderStatus = "RETURNED"
)

type PaymentStatus string

const (
	PaymentPaid   PaymentStatus = "PAID"
	PaymentUnpaid PaymentStatus = "UNPAID"
)

type PaymentMethod string

const (
	PaymentCashOnDelivery PaymentMethod = "CASH_ON_DELIVERY"
	PaymentBankCard       PaymentMethod = "BANK_CARD"
)

type OrderInput struct {
	StoreUUID       string             `json:"storeUuid,omitempty"`
	Customer        OrderCustomerInput `json:"customer"`
	PaymentMethod   PaymentMethod      `json:"paymentMethod"`
	ShippingCarrier string             `json:"shippingCarrier,omitempty"`
	TrackingNumber  string             `json:"trackingNumber,omitempty"`
	ShippingFee     int                `json:"shippingFee,omitempty"`
	InternalNotes   string             `json:"internalNotes,omitempty"`
	Items           []OrderItemInput   `json:"items"`
}

type OrderCustomerInput struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	City    string `json:"city"`
	Notes   string `json:"notes,omitempty"`
}

type OrderItemInput struct {
	ProductUUID string `json:"productUuid"`
	VariantUUID string `json:"variantUuid,omitempty"`
	Quantity    int    `json:"quantity"`
}
type Order struct {
	UUID               string       `json:"uuid"`
	OrderNumber        string       `json:"orderNumber"`
	Customer           *Customer    `json:"customer"`
	Status             OrderStatus  `json:"status"`
	PaymentMethod      string       `json:"paymentMethod"`
	PaymentStatus      string       `json:"paymentStatus"`
	ShippingCarrier    *string      `json:"shippingCarrier"`
	TrackingNumber     *string      `json:"trackingNumber"`
	ShippingFee        int          `json:"shippingFee"`
	ShippingFeeAmount  string       `json:"shippingFeeAmount"`
	ItemsTotal         int          `json:"itemsTotal"`
	ItemsTotalAmount   string       `json:"itemsTotalAmount"`
	TotalAmount        int          `json:"totalAmount"`
	TotalAmountDecimal string       `json:"totalAmountDecimal"`
	UnitPriceAmount    string       `json:"unitPriceAmount"`
	CountryCode        *string      `json:"countryCode"`
	SubdivisionCode    *string      `json:"subdivisionCode"`
	SubdivisionName    *string      `json:"subdivisionName"`
	CurrencyCode       *string      `json:"currencyCode"`
	Market             *OrderMarket `json:"market"`
	InternalNotes      *string      `json:"internalNotes"`
	Items              []OrderItem  `json:"items"`
	CreatedAt          *string      `json:"createdAt"`
	UpdatedAt          *string      `json:"updatedAt"`
}

type OrderMarket struct {
	ID                int     `json:"id"`
	CountryCode       *string `json:"countryCode"`
	SubdivisionCode   *string `json:"subdivisionCode"`
	SubdivisionName   *string `json:"subdivisionName"`
	CurrencyCode      *string `json:"currencyCode"`
	UnitPriceAmount   string  `json:"unitPriceAmount"`
	ShippingFeeAmount string  `json:"shippingFeeAmount"`
	ItemsTotalAmount  string  `json:"itemsTotalAmount"`
	TotalAmount       string  `json:"totalAmount"`
}
type OrderItem struct {
	UUID             string  `json:"uuid"`
	ProductUUID      *string `json:"productUuid"`
	VariantUUID      *string `json:"variantUuid"`
	VariantSKU       *string `json:"variantSku"`
	VariantAttribute *string `json:"variantAttribute"`
	VariantColor     *string `json:"variantColor"`
	VariantSize      *string `json:"variantSize"`
	TitleFr          *string `json:"titleFr"`
	TitleAr          *string `json:"titleAr"`
	TitleEn          *string `json:"titleEn"`
	UnitPrice        int     `json:"unitPrice"`
	Quantity         int     `json:"quantity"`
	Subtotal         int     `json:"subtotal"`
}
type OrdersResource struct{ client *Client }

func (r *OrdersResource) List(ctx context.Context, query Query) (Page[Order], error) {
	data, meta, err := apiRequest[struct {
		Orders []Order `json:"orders"`
	}](r.client, ctx, http.MethodGet, "/orders", query, nil, true)
	return Page[Order]{Items: data.Orders, PaginationMeta: meta}, err
}
func (r *OrdersResource) Get(ctx context.Context, uuid string) (Order, error) {
	data, _, err := apiRequest[struct {
		Order Order `json:"order"`
	}](r.client, ctx, http.MethodGet, "/orders/"+url.PathEscape(uuid), nil, nil, true)
	return data.Order, err
}
func (r *OrdersResource) Create(ctx context.Context, input OrderInput) (Order, error) {
	data, _, err := apiRequest[struct {
		Order Order `json:"order"`
	}](r.client, ctx, http.MethodPost, "/orders", nil, input, true)
	return data.Order, err
}
func (r *OrdersResource) UpdateStatus(ctx context.Context, uuid string, status OrderStatus) (Order, error) {
	data, _, err := apiRequest[struct {
		Order Order `json:"order"`
	}](r.client, ctx, http.MethodPatch, "/orders/"+url.PathEscape(uuid)+"/status", nil, map[string]OrderStatus{"status": status}, true)
	return data.Order, err
}
func (r *OrdersResource) UpdateTracking(ctx context.Context, uuid string, carrier string, trackingNumber *string) (Order, error) {
	data, _, err := apiRequest[struct {
		Order Order `json:"order"`
	}](r.client, ctx, http.MethodPatch, "/orders/"+url.PathEscape(uuid)+"/tracking", nil, map[string]any{"carrier": carrier, "trackingNumber": trackingNumber}, true)
	return data.Order, err
}
func (r *OrdersResource) UpdatePaymentStatus(ctx context.Context, uuid string, status PaymentStatus) (Order, error) {
	data, _, err := apiRequest[struct {
		Order Order `json:"order"`
	}](r.client, ctx, http.MethodPatch, "/orders/"+url.PathEscape(uuid)+"/payment-status", nil, map[string]string{"paymentStatus": string(status)}, true)
	return data.Order, err
}
func (r *OrdersResource) Cancel(ctx context.Context, uuid string) (Order, error) {
	data, _, err := apiRequest[struct {
		Order Order `json:"order"`
	}](r.client, ctx, http.MethodDelete, "/orders/"+url.PathEscape(uuid), nil, nil, true)
	return data.Order, err
}

type Customer struct {
	UUID             string  `json:"uuid"`
	Name             string  `json:"name"`
	Phone            string  `json:"phone"`
	Address          *string `json:"address"`
	City             *string `json:"city"`
	TotalOrdersCount int     `json:"totalOrdersCount"`
	Orders           []Order `json:"orders,omitempty"`
}
type CustomersResource struct{ client *Client }

func (r *CustomersResource) List(ctx context.Context, query Query) (Page[Customer], error) {
	data, meta, err := apiRequest[struct {
		Customers []Customer `json:"customers"`
	}](r.client, ctx, http.MethodGet, "/customers", query, nil, true)
	return Page[Customer]{Items: data.Customers, PaginationMeta: meta}, err
}
func (r *CustomersResource) Get(ctx context.Context, uuid string) (Customer, error) {
	data, _, err := apiRequest[struct {
		Customer Customer `json:"customer"`
	}](r.client, ctx, http.MethodGet, "/customers/"+url.PathEscape(uuid), nil, nil, true)
	return data.Customer, err
}

type StatsOverview struct {
	TotalRevenue          int     `json:"totalRevenue"`
	TodaySales            int     `json:"todaySales"`
	TotalOrdersCount      int     `json:"totalOrdersCount"`
	PendingOrdersCount    int     `json:"pendingOrdersCount"`
	ProcessingOrdersCount int     `json:"processingOrdersCount"`
	ShippedOrdersCount    int     `json:"shippedOrdersCount"`
	DeliveredOrdersCount  int     `json:"deliveredOrdersCount"`
	CancelledOrdersCount  int     `json:"cancelledOrdersCount"`
	DeliveryRate          float64 `json:"deliveryRate"`
	TotalProductsCount    int     `json:"totalProductsCount"`
	TotalCustomersCount   int     `json:"totalCustomersCount"`
	LowStockProductsCount int     `json:"lowStockProductsCount"`
}
type StatsResource struct{ client *Client }

func (r *StatsResource) Overview(ctx context.Context) (StatsOverview, error) {
	data, _, err := apiRequest[StatsOverview](r.client, ctx, http.MethodGet, "/stats/overview", nil, nil, true)
	return data, err
}

type NotificationEndpoint struct {
	ID        int     `json:"id"`
	Channel   string  `json:"channel"`
	Enabled   bool    `json:"enabled"`
	CreatedAt *string `json:"createdAt"`
}
type NotificationChannel string

const (
	NotificationWebPush    NotificationChannel = "web_push"
	NotificationMobilePush NotificationChannel = "mobile_push"
)

type NotificationEndpointInput struct {
	Channel  NotificationChannel `json:"channel"`
	Endpoint string              `json:"endpoint"`
	Metadata map[string]any      `json:"metadata,omitempty"`
}
type NotificationsResource struct{ client *Client }

func (r *NotificationsResource) ListEndpoints(ctx context.Context) ([]NotificationEndpoint, error) {
	data, _, err := apiRequest[struct {
		Endpoints []NotificationEndpoint `json:"endpoints"`
	}](r.client, ctx, http.MethodGet, "/notifications/endpoints", nil, nil, true)
	return data.Endpoints, err
}
func (r *NotificationsResource) RegisterEndpoint(ctx context.Context, input NotificationEndpointInput) (NotificationEndpoint, error) {
	data, _, err := apiRequest[struct {
		Endpoint NotificationEndpoint `json:"endpoint"`
	}](r.client, ctx, http.MethodPost, "/notifications/endpoints", nil, input, true)
	return data.Endpoint, err
}
func (r *NotificationsResource) RemoveEndpoint(ctx context.Context, input NotificationEndpointInput) error {
	_, _, err := apiRequest[map[string]any](r.client, ctx, http.MethodDelete, "/notifications/endpoints", nil, input, true)
	return err
}
