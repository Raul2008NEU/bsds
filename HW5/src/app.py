"""
Product API - Implementation based on OpenAPI 3.0.3 specification.

Endpoints:
  GET  /products/{productId}          - Retrieve a product by ID
  POST /products/{productId}/details  - Add/update product details
"""

from flask import Flask, request, jsonify

app = Flask(__name__)

# In-memory product store (HashMap: product_id -> product dict)
products: dict[int, dict] = {}

REQUIRED_FIELDS = {"product_id", "sku", "manufacturer", "category_id", "weight", "some_other_id"}


def error_response(error_code: str, message: str, details: str = None, status: int = 400):
    """Build a standardized error response matching the Error schema."""
    body = {"error": error_code, "message": message}
    if details:
        body["details"] = details
    return jsonify(body), status


def validate_product(data: dict, path_product_id: int) -> str | None:
    """
    Validate product payload against the Product schema.
    Returns an error message string if invalid, or None if valid.
    """
    # Check all required fields are present
    missing = REQUIRED_FIELDS - set(data.keys())
    if missing:
        return f"Missing required fields: {', '.join(sorted(missing))}"

    # product_id must match the path parameter
    if data.get("product_id") != path_product_id:
        return "product_id in body must match the productId in the URL path"

    # Integer field validations
    int_fields_min1 = ["product_id", "category_id", "some_other_id"]
    for field in int_fields_min1:
        val = data.get(field)
        if not isinstance(val, int) or val < 1:
            return f"{field} must be a positive integer (minimum 1)"

    # weight: integer >= 0
    weight = data.get("weight")
    if not isinstance(weight, int) or weight < 0:
        return "weight must be a non-negative integer"

    # String field validations
    sku = data.get("sku")
    if not isinstance(sku, str) or len(sku) < 1 or len(sku) > 100:
        return "sku must be a string between 1 and 100 characters"

    manufacturer = data.get("manufacturer")
    if not isinstance(manufacturer, str) or len(manufacturer) < 1 or len(manufacturer) > 200:
        return "manufacturer must be a string between 1 and 200 characters"

    return None


@app.route("/products/<int:product_id>", methods=["GET"])
def get_product(product_id: int):
    """GET /products/{productId} - Retrieve a product by ID."""
    if product_id < 1:
        return error_response("INVALID_INPUT", "Invalid product ID", "Product ID must be a positive integer", 400)

    product = products.get(product_id)
    if product is None:
        return error_response("NOT_FOUND", "Product not found", f"No product with ID {product_id}", 404)

    return jsonify(product), 200


@app.route("/products/<int:product_id>/details", methods=["POST"])
def add_product_details(product_id: int):
    """POST /products/{productId}/details - Add or update product details."""
    if product_id < 1:
        return error_response("INVALID_INPUT", "Invalid product ID", "Product ID must be a positive integer", 400)

    # Parse JSON body
    data = request.get_json(silent=True)
    if data is None:
        return error_response("INVALID_INPUT", "Request body must be valid JSON", status=400)

    # Validate payload
    validation_error = validate_product(data, product_id)
    if validation_error:
        return error_response("INVALID_INPUT", "The provided input data is invalid", validation_error, 400)

    # Store the product (create or update)
    products[product_id] = {
        "product_id": data["product_id"],
        "sku": data["sku"],
        "manufacturer": data["manufacturer"],
        "category_id": data["category_id"],
        "weight": data["weight"],
        "some_other_id": data["some_other_id"],
    }

    return "", 204


@app.errorhandler(404)
def not_found(e):
    return error_response("NOT_FOUND", "The requested resource was not found", status=404)


@app.errorhandler(405)
def method_not_allowed(e):
    return error_response("METHOD_NOT_ALLOWED", "HTTP method not allowed for this endpoint", status=405)


@app.errorhandler(500)
def internal_error(e):
    return error_response("INTERNAL_ERROR", "An unexpected error occurred", status=500)


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)