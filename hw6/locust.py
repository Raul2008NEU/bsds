from locust import task, constant
from locust.contrib.fasthttp import FastHttpUser
import random

# Search terms that will hit different categories / brands
SEARCH_TERMS = [
    "electronics",
    "books",
    "home",
    "sports",
    "alpha",
    "beta",
    "gamma",
    "clothing",
    "toys",
    "health",
]


class ProductSearchUser(FastHttpUser):
    # No wait between requests — hammer the service as hard as possible
    wait_time = constant(0)

    @task
    def search_products(self):
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
            f"/search?q={term}",
            catch_response=True,
            name="/search",
        ) as resp:
            if resp.status_code == 200:
                data = resp.json()
                # Basic sanity check: response must have the right shape
                if "products" not in data or "total_found" not in data:
                    resp.failure("Response missing expected fields")
                else:
                    resp.success()
            else:
                resp.failure(f"HTTP {resp.status_code}")

    @task(1)  # Lower weight — occasional health checks
    def health_check(self):
        with self.client.get("/health", catch_response=True, name="/health") as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Health check failed: {resp.status_code}")