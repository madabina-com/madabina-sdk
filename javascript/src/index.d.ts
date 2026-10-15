export interface TokenPair { accessToken: string; refreshToken: string; expiresIn?: number; tokenType?: string }
export interface ApiPage<T> { items: T[]; page: number; perPage: number; total: number; lastPage: number }
export type Query = Record<string, string | number | boolean | null | undefined>;
export interface User { uuid: string; email: string; storeName?: string | null; phone?: string | null; currency: string; defaultLanguage: string; stores?: Store[] }
export interface Store { uuid: string; name: string; slug: string; phone?: string | null; currency: string; defaultLanguage: string; markets?: Array<Record<string, unknown>> }
export interface ProductVariantInput { uuid?: string; sku: string; attribute?: string; color?: string; size?: string; customAttribute?: string; price: number; compareAtPrice?: number; stockQuantity: number }
export interface ProductVariant { uuid: string; sku?: string | null; attribute?: string | null; color?: string | null; size?: string | null; customAttribute?: string | null; mediaId?: number | null; price: number; compareAtPrice?: number | null; stockQuantity: number; isActive: boolean; marketPrices?: Array<Record<string, unknown>> }
export interface Product { uuid: string; storeUuid?: string | null; sku?: string | null; titleFr: string; titleAr?: string | null; titleEn: string; descriptionFr?: string | null; descriptionAr?: string | null; descriptionEn?: string | null; category?: string | null; categories?: Array<Record<string, unknown>>; price: number; productType: 'simple' | 'variable'; compareAtPrice?: number | null; costPrice?: number | null; stockQuantity: number; minStockAlert?: number | null; isVisibleOnStore: boolean; imageUrl?: string | null; createdAt?: string | null; updatedAt?: string | null; marketPrices?: Array<Record<string, unknown>>; variants?: ProductVariant[] }
export interface ProductInput { storeUuid?: string; sku?: string; titleFr: string; titleAr: string; titleEn: string; descriptionFr?: string; descriptionAr?: string; descriptionEn?: string; category: 'ELECTRONICS' | 'FASHION' | 'BEAUTY' | 'HOME' | 'CRAFTS'; price: number; compareAtPrice?: number; productType?: 'simple' | 'variable'; costPrice?: number; stockQuantity: number; minStockAlert?: number; isVisibleOnStore?: boolean; imageUrl?: string; variants?: ProductVariantInput[] }
export type OrderStatus = 'PENDING' | 'CONFIRMED' | 'PROCESSING' | 'SHIPPED' | 'DELIVERED' | 'CANCELLED' | 'RETURNED';
export interface OrderInput { storeUuid?: string; customer: { name: string; phone: string; address: string; city: string; notes?: string }; paymentMethod: 'CASH_ON_DELIVERY' | 'BANK_CARD'; shippingCarrier?: string; trackingNumber?: string; shippingFee?: number; internalNotes?: string; items: Array<{ productUuid: string; variantUuid?: string; quantity: number }> }
export interface OrderItem { uuid: string; productUuid?: string | null; variantUuid?: string | null; variantSku?: string | null; variantAttribute?: string | null; variantColor?: string | null; variantSize?: string | null; titleFr?: string | null; titleAr?: string | null; titleEn?: string | null; unitPrice: number; quantity: number; subtotal: number }
export interface Order { uuid: string; orderNumber: string; customer?: Customer | null; status: OrderStatus; paymentMethod: 'CASH_ON_DELIVERY' | 'BANK_CARD'; paymentStatus: 'PAID' | 'UNPAID'; shippingCarrier?: string | null; trackingNumber?: string | null; shippingFee: number; shippingFeeAmount?: string; itemsTotal: number; itemsTotalAmount?: string; totalAmount: number; totalAmountDecimal?: string; unitPriceAmount?: string; countryCode?: string | null; subdivisionCode?: string | null; subdivisionName?: string | null; currencyCode?: string | null; market?: Record<string, unknown> | null; internalNotes?: string | null; items: OrderItem[]; createdAt?: string | null; updatedAt?: string | null }
export interface Customer { uuid: string; name: string; phone: string; address?: string | null; city?: string | null; notes?: string | null; totalOrdersCount: number; orders?: Order[] }
export interface NotificationEndpoint { id: number; channel: 'web_push' | 'mobile_push'; enabled: boolean; createdAt?: string | null }
export interface MadabinaClientOptions { baseUrl?: string; accessToken?: string; refreshToken?: string; fetch?: typeof globalThis.fetch }
export interface LoginCredentials { email: string; password: string }
export class MadabinaApiError extends Error { readonly status: number; readonly details?: unknown; constructor(message: string, status: number, details?: unknown) }
export class MadabinaClient {
  constructor(options?: MadabinaClientOptions);
  readonly baseUrl: string;
  readonly auth: {
    login(credentials: LoginCredentials): Promise<{ tokens: TokenPair; user: User }>;
    refresh(): Promise<TokenPair>;
    me(): Promise<User>;
    logout(): Promise<void>;
  };
  readonly products: {
    list(query?: Query): Promise<ApiPage<Product>>; get(uuid: string): Promise<Product>; create(payload: ProductInput): Promise<Product>; update(uuid: string, payload: ProductInput): Promise<Product>; patch(uuid: string, payload: ProductInput): Promise<Product>;
    updateStock(uuid: string, payload: { delta?: number; newQuantity?: number; variantUuid?: string }): Promise<Product>; updateVisibility(uuid: string, isVisible: boolean): Promise<Product>; delete(uuid: string): Promise<void>;
  };
  readonly orders: {
    list(query?: Query): Promise<ApiPage<Order>>; get(uuid: string): Promise<Order>; create(payload: OrderInput): Promise<Order>; updateStatus(uuid: string, status: OrderStatus): Promise<Order>;
    updateTracking(uuid: string, payload: { carrier: string; trackingNumber?: string | null }): Promise<Order>; updatePaymentStatus(uuid: string, status: 'PAID' | 'UNPAID'): Promise<Order>; cancel(uuid: string): Promise<Order>;
  };
  readonly customers: { list(query?: Query): Promise<ApiPage<Customer>>; get(uuid: string): Promise<Customer> };
  readonly stats: { overview(): Promise<Record<string, number>> };
  readonly notifications: {
    listEndpoints(): Promise<NotificationEndpoint[]>;
    registerEndpoint(payload: { channel: 'web_push' | 'mobile_push'; endpoint: string; metadata?: Record<string, unknown> }): Promise<NotificationEndpoint>;
    removeEndpoint(payload: { channel: 'web_push' | 'mobile_push'; endpoint: string }): Promise<void>;
  };
  getTokens(): Partial<TokenPair>;
  setTokens(tokens: Partial<TokenPair>): void;
  clearTokens(): void;
}
