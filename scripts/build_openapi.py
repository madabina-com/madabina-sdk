#!/usr/bin/env python3
"""Build the canonical REST contract and its Laravel-served copy."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
WEB_APP = ROOT.parent / "web-app"
SDK_SPEC = ROOT / "openapi.json"
LARAVEL_SPEC = WEB_APP / "resources" / "openapi" / "madabina-rest.json"


def ref(name: str) -> dict:
    return {"$ref": f"#/components/schemas/{name}"}


def envelope(data: dict, *, paginated: bool = False) -> dict:
    return {
        "type": "object",
        "required": ["success", "data", "meta"],
        "properties": {
            "success": {"type": "boolean", "enum": [True]},
            "data": data,
            "meta": ref("PaginationMeta") if paginated else {"type": "object", "additionalProperties": True},
        },
    }


def data_object(**properties: dict) -> dict:
    return {"type": "object", "properties": properties}


def operation(summary: str, tag: str, data_schema: dict, *, status: str = "200", body: str | None = None, query: list[dict] | None = None, path_uuid: bool = False, public: bool = False) -> dict:
    responses = {
        status: {"description": "Successful response", "content": {"application/json": {"schema": envelope(data_schema, paginated=query is not None)}}},
        "401": {"description": "Missing, invalid, or expired credentials", "content": {"application/json": {"schema": ref("ApiError")}}},
        "422": {"description": "Validation failed", "content": {"application/json": {"schema": ref("ApiError")}}},
    }
    if path_uuid:
        responses["404"] = {"description": "Resource not found", "content": {"application/json": {"schema": ref("ApiError")}}}
    result = {
        "tags": [tag],
        "summary": summary,
        "responses": responses,
    }
    if not public:
        result["security"] = [{"bearerAuth": []}]
    if path_uuid:
        result["parameters"] = [{"name": "uuid", "in": "path", "required": True, "schema": {"type": "string", "format": "uuid"}}]
    if query:
        result.setdefault("parameters", []).extend(query)
    if body:
        result["requestBody"] = {
            "required": True,
            "content": {"application/json": {"schema": ref(body)}},
        }
    return result


def query(name: str, schema: dict, description: str | None = None) -> dict:
    return {"name": name, "in": "query", "required": False, "description": description or name, "schema": schema}


def build_spec() -> dict:
    common_page = [
        query("page", {"type": "integer", "minimum": 1, "default": 1}),
        query("perPage", {"type": "integer", "minimum": 1, "maximum": 100, "default": 15}),
        query("search", {"type": "string"}),
        query("storeUuid", {"type": "string", "format": "uuid"}, "Select one of the merchant's active stores."),
    ]
    product_query = common_page + [
        query("category", {"type": "string", "enum": ["ELECTRONICS", "FASHION", "BEAUTY", "HOME", "CRAFTS"]}),
        query("isVisible", {"type": "boolean"}),
        query("stockStatus", {"type": "string", "enum": ["low", "out"]}),
    ]
    order_query = common_page + [
        query("status", {"type": "string"}),
        query("dateFrom", {"type": "string", "format": "date"}),
        query("dateTo", {"type": "string", "format": "date"}),
    ]
    customer_query = common_page

    paths = {
        "/auth/login": {"post": operation("Merchant login", "Authentication", data_object(accessToken={"type": "string"}, refreshToken={"type": "string"}, expiresIn={"type": "integer"}, tokenType={"type": "string"}, user=ref("User")), body="LoginRequest", public=True)},
        "/auth/refresh": {"post": operation("Rotate an access and refresh token pair", "Authentication", ref("TokenPair"), body="RefreshTokenRequest", public=True)},
        "/auth/me": {"get": operation("Get the authenticated merchant profile", "Authentication", data_object(user=ref("User")))},
        "/auth/logout": {"post": operation("Revoke the supplied refresh token", "Authentication", data_object(), body="RefreshTokenRequest")},
        "/stats/overview": {"get": operation("Get store overview metrics", "Statistics", ref("StatsOverview"))},
        "/notifications/endpoints": {
            "get": operation("List registered Web Push and mobile push endpoints", "Notifications", data_object(endpoints={"type": "array", "items": ref("NotificationEndpoint")})),
            "post": operation("Register a Web Push or mobile push endpoint", "Notifications", data_object(endpoint=ref("RegisteredNotificationEndpoint")), status="201", body="RegisterNotificationEndpoint"),
            "delete": operation("Remove a Web Push or mobile push endpoint", "Notifications", data_object(), body="RemoveNotificationEndpoint"),
        },
        "/products": {
            "get": operation("List products", "Products", data_object(products={"type": "array", "items": ref("Product")}), query=product_query),
            "post": operation("Create a product", "Products", data_object(product=ref("Product")), status="201", body="ProductInput"),
        },
        "/products/{uuid}": {
            "get": operation("Get a product", "Products", data_object(product=ref("Product")), path_uuid=True),
            "put": operation("Replace a product", "Products", data_object(product=ref("Product")), path_uuid=True, body="ProductInput"),
            "patch": operation("Update a product", "Products", data_object(product=ref("Product")), path_uuid=True, body="ProductInput"),
            "delete": operation("Delete a product", "Products", data_object(), path_uuid=True),
        },
        "/products/{uuid}/stock": {"patch": operation("Adjust product stock", "Products", data_object(product=ref("Product")), path_uuid=True, body="StockInput")},
        "/products/{uuid}/visibility": {"patch": operation("Update product store visibility", "Products", data_object(product=ref("Product")), path_uuid=True, body="VisibilityInput")},
        "/orders": {
            "get": operation("List orders", "Orders", data_object(orders={"type": "array", "items": ref("Order")}), query=order_query),
            "post": operation("Create an order", "Orders", data_object(order=ref("Order")), status="201", body="OrderInput"),
        },
        "/orders/{uuid}": {
            "get": operation("Get an order", "Orders", data_object(order=ref("Order")), path_uuid=True),
            "delete": operation("Cancel an order", "Orders", data_object(order=ref("Order")), path_uuid=True),
        },
        "/orders/{uuid}/status": {"patch": operation("Update order status", "Orders", data_object(order=ref("Order")), path_uuid=True, body="OrderStatusInput")},
        "/orders/{uuid}/tracking": {"patch": operation("Update order tracking", "Orders", data_object(order=ref("Order")), path_uuid=True, body="OrderTrackingInput")},
        "/orders/{uuid}/payment-status": {"patch": operation("Update order payment status", "Orders", data_object(order=ref("Order")), path_uuid=True, body="PaymentStatusInput")},
        "/customers": {"get": operation("List customers", "Customers", data_object(customers={"type": "array", "items": ref("Customer")}), query=customer_query)},
        "/customers/{uuid}": {"get": operation("Get a customer and their orders", "Customers", data_object(customer=ref("Customer")), path_uuid=True)},
    }

    schemas = {
        "ApiError": {"type": "object", "required": ["success", "message"], "properties": {"success": {"type": "boolean", "enum": [False]}, "message": {"type": "string"}, "statusCode": {"type": "integer"}, "details": {"type": "object", "additionalProperties": {"oneOf": [{"type": "string"}, {"type": "array", "items": {"type": "string"}}]}}}},
        "TokenPair": {"type": "object", "required": ["accessToken", "refreshToken"], "properties": {"accessToken": {"type": "string"}, "refreshToken": {"type": "string"}, "expiresIn": {"type": "integer"}, "tokenType": {"type": "string", "default": "Bearer"}}},
        "LoginRequest": {"type": "object", "required": ["email", "password"], "properties": {"email": {"type": "string", "format": "email"}, "password": {"type": "string", "format": "password"}}},
        "RefreshTokenRequest": {"type": "object", "required": ["refreshToken"], "properties": {"refreshToken": {"type": "string"}}},
        "Store": {"type": "object", "required": ["uuid", "name", "slug", "currency", "defaultLanguage"], "properties": {"uuid": {"type": "string", "format": "uuid"}, "name": {"type": "string"}, "slug": {"type": "string"}, "phone": {"type": ["string", "null"]}, "currency": {"type": "string"}, "defaultLanguage": {"type": "string"}, "markets": {"type": "array", "items": {"type": "object", "additionalProperties": True}}}},
        "User": {"type": "object", "required": ["uuid", "email", "currency", "defaultLanguage", "stores"], "properties": {"uuid": {"type": "string", "format": "uuid"}, "email": {"type": "string", "format": "email"}, "storeName": {"type": ["string", "null"]}, "phone": {"type": ["string", "null"]}, "currency": {"type": "string"}, "defaultLanguage": {"type": "string"}, "stores": {"type": "array", "items": ref("Store")}}},
        "ProductVariant": {"type": "object", "additionalProperties": True, "properties": {"uuid": {"type": "string", "format": "uuid"}, "sku": {"type": "string", "nullable": True}, "attribute": {"type": "string", "nullable": True}, "color": {"type": "string", "nullable": True}, "size": {"type": "string", "nullable": True}, "customAttribute": {"type": "string", "nullable": True}, "mediaId": {"type": "integer", "nullable": True}, "price": {"type": "integer"}, "compareAtPrice": {"type": "integer", "nullable": True}, "stockQuantity": {"type": "integer"}, "isActive": {"type": "boolean"}, "marketPrices": {"type": "array", "items": {"type": "object", "additionalProperties": True}}}},
        "ProductVariantInput": {"type": "object", "required": ["sku", "price", "stockQuantity"], "properties": {"uuid": {"type": "string", "format": "uuid"}, "sku": {"type": "string", "maxLength": 80}, "attribute": {"type": "string", "maxLength": 255}, "color": {"type": "string", "maxLength": 100}, "size": {"type": "string", "maxLength": 100}, "customAttribute": {"type": "string", "maxLength": 255}, "price": {"type": "integer", "minimum": 0}, "compareAtPrice": {"type": "integer", "minimum": 0}, "stockQuantity": {"type": "integer", "minimum": 0}}},
        "Product": {"type": "object", "additionalProperties": True, "required": ["uuid", "titleFr", "titleEn", "price", "stockQuantity", "isVisibleOnStore"], "properties": {"uuid": {"type": "string", "format": "uuid"}, "storeUuid": {"type": "string", "format": "uuid", "nullable": True}, "sku": {"type": "string", "nullable": True}, "titleFr": {"type": "string"}, "titleAr": {"type": "string", "nullable": True}, "titleEn": {"type": "string"}, "descriptionFr": {"type": "string", "nullable": True}, "descriptionAr": {"type": "string", "nullable": True}, "descriptionEn": {"type": "string", "nullable": True}, "category": {"type": "string", "nullable": True}, "categories": {"type": "array", "items": {"type": "object", "additionalProperties": True}}, "price": {"type": "integer"}, "productType": {"type": "string", "enum": ["simple", "variable"]}, "compareAtPrice": {"type": "integer", "nullable": True}, "costPrice": {"type": "integer", "nullable": True}, "stockQuantity": {"type": "integer"}, "minStockAlert": {"type": "integer", "nullable": True}, "isVisibleOnStore": {"type": "boolean"}, "imageUrl": {"type": "string", "nullable": True}, "createdAt": {"type": "string", "format": "date-time", "nullable": True}, "updatedAt": {"type": "string", "format": "date-time", "nullable": True}, "marketPrices": {"type": "array", "items": {"type": "object", "additionalProperties": True}}, "variants": {"type": "array", "items": ref("ProductVariant")}}},
        "ProductInput": {"type": "object", "required": ["titleFr", "titleAr", "titleEn", "category", "price", "stockQuantity"], "properties": {"storeUuid": {"type": "string", "format": "uuid"}, "sku": {"type": "string", "maxLength": 80}, "titleFr": {"type": "string", "maxLength": 255}, "titleAr": {"type": "string", "maxLength": 255}, "titleEn": {"type": "string", "maxLength": 255}, "descriptionFr": {"type": "string"}, "descriptionAr": {"type": "string"}, "descriptionEn": {"type": "string"}, "category": {"type": "string", "enum": ["ELECTRONICS", "FASHION", "BEAUTY", "HOME", "CRAFTS"]}, "price": {"type": "integer", "minimum": 0}, "compareAtPrice": {"type": "integer", "minimum": 0}, "productType": {"type": "string", "enum": ["simple", "variable"]}, "costPrice": {"type": "integer", "minimum": 0}, "stockQuantity": {"type": "integer", "minimum": 0}, "minStockAlert": {"type": "integer", "minimum": 0}, "isVisibleOnStore": {"type": "boolean"}, "imageUrl": {"type": "string", "maxLength": 2048}, "variants": {"type": "array", "items": ref("ProductVariantInput")}}},
        "OrderItem": {"type": "object", "properties": {"uuid": {"type": "string", "format": "uuid"}, "productUuid": {"type": ["string", "null"], "format": "uuid"}, "variantUuid": {"type": ["string", "null"], "format": "uuid"}, "titleFr": {"type": ["string", "null"]}, "titleAr": {"type": ["string", "null"]}, "titleEn": {"type": ["string", "null"]}, "unitPrice": {"type": "integer"}, "quantity": {"type": "integer"}, "subtotal": {"type": "integer"}}},
        "Order": {"type": "object", "additionalProperties": True, "required": ["uuid", "orderNumber", "status", "paymentStatus", "totalAmount", "items"], "properties": {"uuid": {"type": "string", "format": "uuid"}, "orderNumber": {"type": "string"}, "customer": {"allOf": [ref("Customer")], "nullable": True}, "status": {"type": "string", "enum": ["PENDING", "CONFIRMED", "PROCESSING", "SHIPPED", "DELIVERED", "CANCELLED", "RETURNED"]}, "paymentMethod": {"type": "string", "enum": ["CASH_ON_DELIVERY", "BANK_CARD"]}, "paymentStatus": {"type": "string", "enum": ["PAID", "UNPAID"]}, "shippingCarrier": {"type": "string", "nullable": True}, "trackingNumber": {"type": "string", "nullable": True}, "shippingFee": {"type": "integer"}, "shippingFeeAmount": {"type": "string"}, "itemsTotal": {"type": "integer"}, "itemsTotalAmount": {"type": "string"}, "totalAmount": {"type": "integer"}, "totalAmountDecimal": {"type": "string"}, "unitPriceAmount": {"type": "string"}, "countryCode": {"type": "string", "nullable": True}, "subdivisionCode": {"type": "string", "nullable": True}, "subdivisionName": {"type": "string", "nullable": True}, "currencyCode": {"type": "string", "nullable": True}, "market": {"type": "object", "nullable": True, "additionalProperties": True}, "internalNotes": {"type": "string", "nullable": True}, "items": {"type": "array", "items": ref("OrderItem")}, "createdAt": {"type": "string", "format": "date-time", "nullable": True}, "updatedAt": {"type": "string", "format": "date-time", "nullable": True}}},
        "OrderInput": {"type": "object", "required": ["customer", "paymentMethod", "items"], "properties": {"storeUuid": {"type": "string", "format": "uuid"}, "customer": {"type": "object", "required": ["name", "phone", "address", "city"], "properties": {"name": {"type": "string", "maxLength": 255}, "phone": {"type": "string", "maxLength": 30}, "address": {"type": "string", "maxLength": 1000}, "city": {"type": "string", "maxLength": 100}, "notes": {"type": "string", "maxLength": 1000}}}, "paymentMethod": {"type": "string", "enum": ["CASH_ON_DELIVERY", "BANK_CARD"]}, "shippingCarrier": {"type": "string", "maxLength": 255}, "trackingNumber": {"type": "string", "maxLength": 255}, "shippingFee": {"type": "integer", "minimum": 0}, "internalNotes": {"type": "string", "maxLength": 2000}, "items": {"type": "array", "minItems": 1, "items": {"type": "object", "required": ["productUuid", "quantity"], "properties": {"productUuid": {"type": "string", "format": "uuid"}, "variantUuid": {"type": "string", "format": "uuid"}, "quantity": {"type": "integer", "minimum": 1}}}}}},
        "Customer": {"type": "object", "required": ["uuid", "name", "phone", "totalOrdersCount"], "properties": {"uuid": {"type": "string", "format": "uuid"}, "name": {"type": "string"}, "phone": {"type": "string"}, "address": {"type": ["string", "null"]}, "city": {"type": ["string", "null"]}, "notes": {"type": ["string", "null"]}, "totalOrdersCount": {"type": "integer"}, "orders": {"type": "array", "items": ref("Order")}}},
        "StatsOverview": {"type": "object", "properties": {"totalRevenue": {"type": "integer"}, "todaySales": {"type": "integer"}, "totalOrdersCount": {"type": "integer"}, "pendingOrdersCount": {"type": "integer"}, "processingOrdersCount": {"type": "integer"}, "shippedOrdersCount": {"type": "integer"}, "deliveredOrdersCount": {"type": "integer"}, "cancelledOrdersCount": {"type": "integer"}, "deliveryRate": {"type": "number"}, "totalProductsCount": {"type": "integer"}, "totalCustomersCount": {"type": "integer"}, "lowStockProductsCount": {"type": "integer"}}},
        "NotificationEndpoint": {"type": "object", "required": ["id", "channel", "enabled"], "properties": {"id": {"type": "integer"}, "channel": {"type": "string", "enum": ["web_push", "mobile_push"]}, "enabled": {"type": "boolean"}, "createdAt": {"type": ["string", "null"], "format": "date-time"}}},
        "RegisteredNotificationEndpoint": {"type": "object", "required": ["id", "channel", "enabled"], "properties": {"id": {"type": "integer"}, "channel": {"type": "string"}, "enabled": {"type": "boolean"}}},
        "RegisterNotificationEndpoint": {"type": "object", "required": ["channel", "endpoint"], "properties": {"channel": {"type": "string", "enum": ["web_push", "mobile_push"]}, "endpoint": {"type": "string", "maxLength": 20000}, "metadata": {"type": "object", "additionalProperties": True}}},
        "RemoveNotificationEndpoint": {"type": "object", "required": ["channel", "endpoint"], "properties": {"channel": {"type": "string", "enum": ["web_push", "mobile_push"]}, "endpoint": {"type": "string", "maxLength": 20000}}},
        "StockInput": {"type": "object", "properties": {"variantUuid": {"type": "string", "format": "uuid"}, "delta": {"type": "integer"}, "newQuantity": {"type": "integer", "minimum": 0}}, "anyOf": [{"required": ["delta"]}, {"required": ["newQuantity"]}]},
        "VisibilityInput": {"type": "object", "required": ["isVisible"], "properties": {"isVisible": {"type": "boolean"}}},
        "OrderStatusInput": {"type": "object", "required": ["status"], "properties": {"status": {"type": "string", "enum": ["PENDING", "CONFIRMED", "PROCESSING", "SHIPPED", "DELIVERED", "CANCELLED", "RETURNED"]}}},
        "OrderTrackingInput": {"type": "object", "required": ["carrier"], "properties": {"carrier": {"type": "string", "maxLength": 255}, "trackingNumber": {"type": "string", "maxLength": 255}}},
        "PaymentStatusInput": {"type": "object", "required": ["paymentStatus"], "properties": {"paymentStatus": {"type": "string", "enum": ["PAID", "UNPAID"]}}},
        "PaginationMeta": {"type": "object", "required": ["page", "perPage", "total", "lastPage"], "properties": {"page": {"type": "integer"}, "perPage": {"type": "integer"}, "total": {"type": "integer"}, "lastPage": {"type": "integer"}}},
    }

    def normalize_openapi_30(value: object) -> object:
        if isinstance(value, list):
            return [normalize_openapi_30(item) for item in value]
        if not isinstance(value, dict):
            return value
        normalized = {key: normalize_openapi_30(item) for key, item in value.items()}
        types = normalized.get("type")
        if isinstance(types, list) and "null" in types:
            non_null = [item for item in types if item != "null"]
            normalized["type"] = non_null[0] if len(non_null) == 1 else non_null
            normalized["nullable"] = True
        return normalized

    paths = normalize_openapi_30(paths)
    schemas = normalize_openapi_30(schemas)
    return {
        "openapi": "3.0.3",
        "info": {"title": "Madabina Mobile API", "version": "1.0.0", "description": "Merchant REST API. All resources are scoped to stores owned by the authenticated merchant."},
        "servers": [{"url": "/api/v1"}],
        "security": [{"bearerAuth": []}],
        "tags": [{"name": name} for name in ["Authentication", "Products", "Orders", "Customers", "Statistics", "Notifications"]],
        "components": {
            "securitySchemes": {"bearerAuth": {"type": "http", "scheme": "bearer", "bearerFormat": "Bearer token"}},
            "schemas": schemas,
        },
        "paths": paths,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail when the tracked copies are missing or stale")
    args = parser.parse_args()
    content = json.dumps(build_spec(), ensure_ascii=False, indent=2) + "\n"
    targets = [SDK_SPEC, LARAVEL_SPEC]
    if args.check:
        stale = [str(path) for path in targets if not path.exists() or path.read_text(encoding="utf-8") != content]
        if stale:
            raise SystemExit("OpenAPI contract copies are stale: " + ", ".join(stale))
        print("OpenAPI contract copies are aligned.")
        return 0
    for path in targets:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
    print("Wrote the SDK contract and Laravel-served OpenAPI document.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
