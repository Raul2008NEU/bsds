"""
Locust load test for Product API using FastHttpUser (geventhttpclient-based).

Identical workload to locust_http.py but uses FastHttpUser which:
- Uses geventhttpclient instead of requests library
- Has significantly lower CPU overhead per request on the CLIENT side
- Allows generating higher request throughput from a single machine
- The difference becomes visible at HIGH concurrency (100+ users)

At LOW concurrency, HttpUser and FastHttpUser perform similarly because
the bottleneck is the server or network, not the client's HTTP library.
"""

import random
from locust import task, between, events
from locust.contrib.fasthttp import FastHttpUser

# Pre-seed product IDs that will exist
PRODUCT_IDS = list(range(1, 101))  # products 1-100
NON_EXISTENT_IDS = list(range(9000, 9050))  # IDs that won't exist


class ProductApiFastHttpUser(FastHttpUser):
    wait_time = between(0.1, 0.5)  # aggressive wait time for stress testing

    def on_start(self):
        """Seed some products when user starts."""
        for pid in random.sample(PRODUCT_IDS, 10):
            self.client.post(
                f"/products/{pid}/details",
                json={
                    "product_id": pid,
                    "sku": f"SKU-{pid:05d}",
                    "manufacturer": f"Manufacturer-{pid % 20}",
                    "category_id": (pid % 10) + 1,
                    "weight": random.randint(100, 5000),
                    "some_other_id": random.randint(1, 999),
                },
                name="/products/{id}/details [seed]",
            )

    # ── READ-HEAVY workload (4 read tasks) ──

    @task(4)
    def get_existing_product(self):
        """GET a product that should exist — most common real-world operation."""
        pid = random.choice(PRODUCT_IDS)
        self.client.get(f"/products/{pid}", name="/products/{id} [hit]")

    @task(1)
    def get_nonexistent_product(self):
        """GET a product that doesn't exist — tests 404 handling."""
        pid = random.choice(NON_EXISTENT_IDS)
        with self.client.get(
            f"/products/{pid}",
            name="/products/{id} [miss/404]",
            catch_response=True,
        ) as resp:
            if resp.status_code == 404:
                resp.success()

    # ── WRITE workload ──

    @task(2)
    def create_or_update_product(self):
        """POST product details — create or update."""
        pid = random.choice(PRODUCT_IDS)
        self.client.post(
            f"/products/{pid}/details",
            json={
                "product_id": pid,
                "sku": f"SKU-{pid:05d}-v{random.randint(1,99)}",
                "manufacturer": f"Mfg-{random.randint(1,50)}",
                "category_id": random.randint(1, 20),
                "weight": random.randint(100, 5000),
                "some_other_id": random.randint(1, 999),
            },
            name="/products/{id}/details [upsert]",
        )

    @task(1)
    def create_product_invalid(self):
        """POST with invalid data — tests 400 handling under load."""
        pid = random.choice(PRODUCT_IDS)
        with self.client.post(
            f"/products/{pid}/details",
            json={"product_id": pid, "sku": "incomplete"},
            name="/products/{id}/details [bad/400]",
            catch_response=True,
        ) as resp:
            if resp.status_code == 400:
                resp.success()