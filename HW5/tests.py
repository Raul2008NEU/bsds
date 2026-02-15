"""
Tests for the Product API.
Run with: python -m pytest tests.py -v
"""

import pytest
from app import app, products


@pytest.fixture
def client():
    app.config["TESTING"] = True
    with app.test_client() as client:
        products.clear()
        yield client


VALID_PRODUCT = {
    "product_id": 1,
    "sku": "ABC-123-XYZ",
    "manufacturer": "Acme Corporation",
    "category_id": 456,
    "weight": 1250,
    "some_other_id": 789,
}


# ──────────────────────────── GET /products/{id} ────────────────────────────

class TestGetProduct:
    def test_get_existing_product(self, client):
        # Given: a product exists in the store
        products[1] = VALID_PRODUCT.copy()

        # When: the client requests the product by ID
        resp = client.get("/products/1")

        # Then: it returns 200 with the correct product data
        assert resp.status_code == 200
        data = resp.get_json()
        assert data["product_id"] == 1
        assert data["sku"] == "ABC-123-XYZ"
        assert data["manufacturer"] == "Acme Corporation"

    def test_get_nonexistent_product_returns_404(self, client):
        # Given: no products exist in the store

        # When: the client requests a product that doesn't exist
        resp = client.get("/products/9999")

        # Then: it returns 404 with a NOT_FOUND error
        assert resp.status_code == 404
        data = resp.get_json()
        assert data["error"] == "NOT_FOUND"

    def test_get_product_invalid_id_string(self, client):
        # Given: no products exist in the store

        # When: the client requests a product with a non-integer ID
        resp = client.get("/products/abc")

        # Then: it returns 404 since the route does not match
        assert resp.status_code == 404


# ──────────────── POST /products/{id}/details ────────────────

class TestAddProductDetails:
    def test_create_product_returns_204(self, client):
        # Given: no products exist in the store

        # When: the client posts valid product details
        resp = client.post("/products/1/details", json=VALID_PRODUCT)

        # Then: it returns 204 and the product is stored
        assert resp.status_code == 204
        assert products[1]["sku"] == "ABC-123-XYZ"

    def test_update_existing_product(self, client):
        # Given: a product already exists
        client.post("/products/1/details", json=VALID_PRODUCT)

        # When: the client posts updated details for the same product
        updated = VALID_PRODUCT.copy()
        updated["manufacturer"] = "New Manufacturer"
        resp = client.post("/products/1/details", json=updated)

        # Then: it returns 204 and the product is updated
        assert resp.status_code == 204
        assert products[1]["manufacturer"] == "New Manufacturer"

    def test_missing_required_field_returns_400(self, client):
        # Given: a payload that is missing the required 'sku' field
        payload = VALID_PRODUCT.copy()
        del payload["sku"]

        # When: the client posts the incomplete payload
        resp = client.post("/products/1/details", json=payload)

        # Then: it returns 400 with an INVALID_INPUT error
        assert resp.status_code == 400
        assert resp.get_json()["error"] == "INVALID_INPUT"

    def test_mismatched_product_id_returns_400(self, client):
        # Given: a payload where product_id doesn't match the URL path
        payload = VALID_PRODUCT.copy()
        payload["product_id"] = 999

        # When: the client posts to /products/1/details
        resp = client.post("/products/1/details", json=payload)

        # Then: it returns 400 due to the ID mismatch
        assert resp.status_code == 400

    def test_invalid_json_body_returns_400(self, client):
        # Given: a request body that is not valid JSON

        # When: the client posts the malformed body
        resp = client.post(
            "/products/1/details",
            data="not json",
            content_type="application/json",
        )

        # Then: it returns 400
        assert resp.status_code == 400

    def test_negative_weight_returns_400(self, client):
        # Given: a payload with a negative weight value
        payload = VALID_PRODUCT.copy()
        payload["weight"] = -5

        # When: the client posts the invalid payload
        resp = client.post("/products/1/details", json=payload)

        # Then: it returns 400 since weight must be >= 0
        assert resp.status_code == 400

    def test_zero_product_id_field_returns_400(self, client):
        # Given: a payload with product_id set to 0 (below minimum of 1)
        payload = VALID_PRODUCT.copy()
        payload["product_id"] = 0

        # When: the client posts to /products/0/details
        resp = client.post("/products/0/details", json=payload)

        # Then: it returns 400 since product_id must be >= 1
        assert resp.status_code == 400

    def test_sku_too_long_returns_400(self, client):
        # Given: a payload with a SKU exceeding 100 characters
        payload = VALID_PRODUCT.copy()
        payload["sku"] = "A" * 101

        # When: the client posts the invalid payload
        resp = client.post("/products/1/details", json=payload)

        # Then: it returns 400 since SKU max length is 100
        assert resp.status_code == 400

    def test_manufacturer_too_long_returns_400(self, client):
        # Given: a payload with a manufacturer name exceeding 200 characters
        payload = VALID_PRODUCT.copy()
        payload["manufacturer"] = "M" * 201

        # When: the client posts the invalid payload
        resp = client.post("/products/1/details", json=payload)

        # Then: it returns 400 since manufacturer max length is 200
        assert resp.status_code == 400


# ──────────────── Integration: POST then GET ────────────────

class TestIntegration:
    def test_create_then_retrieve(self, client):
        # Given: no products exist in the store

        # When: the client creates a product and then retrieves it
        resp = client.post("/products/1/details", json=VALID_PRODUCT)
        assert resp.status_code == 204
        resp = client.get("/products/1")

        # Then: the retrieved product matches what was created
        assert resp.status_code == 200
        data = resp.get_json()
        assert data == VALID_PRODUCT

    def test_create_multiple_products(self, client):
        # Given: no products exist in the store

        # When: the client creates three products
        for pid in [1, 2, 3]:
            p = VALID_PRODUCT.copy()
            p["product_id"] = pid
            resp = client.post(f"/products/{pid}/details", json=p)
            assert resp.status_code == 204

        # Then: all three products are retrievable with correct IDs
        for pid in [1, 2, 3]:
            resp = client.get(f"/products/{pid}")
            assert resp.status_code == 200
            assert resp.get_json()["product_id"] == pid