"""
Load test file for order processing service.

HOW TO RUN:
===========

Test 1 — Sync, Normal Load (5 users):
  locust -f locustfile.py SyncUser --headless -u 5 -r 1 -t 30s --host http://YOUR-ALB-DNS

Test 2 — Sync, Flash Sale (20 users):
  locust -f locustfile.py SyncUser --headless -u 20 -r 10 -t 60s --host http://YOUR-ALB-DNS

Test 3 — Async, Flash Sale (20 users):
  locust -f locustfile.py AsyncUser --headless -u 20 -r 10 -t 60s --host http://YOUR-ALB-DNS

PARAMETERS EXPLAINED:
  -u = total number of concurrent simulated users
  -r = how fast to spawn users (users per second)
  -t = total test duration
  --headless = run without the web UI
  --host = your ALB URL (from terraform output)
"""

from locust import HttpUser, task, between
import random
import json


def make_order():
    """Generate a random order payload."""
    return {
        "customer_id": random.randint(1, 10000),
        "items": [
            {
                "item_id": f"ITEM-{random.randint(100, 999)}",
                "name": "Flash Sale Widget",
                "quantity": random.randint(1, 5),
                "price": round(random.uniform(9.99, 99.99), 2),
            }
        ],
    }


class SyncUser(HttpUser):
    """
    Simulates a customer using the SYNCHRONOUS endpoint.
    Each request blocks until payment completes (3+ seconds).
    """
    # Random wait between 100ms and 500ms between requests
    wait_time = between(0.1, 0.5)

    @task
    def place_sync_order(self):
        self.client.post(
            "/orders/sync",
            json=make_order(),
            headers={"Content-Type": "application/json"},
            timeout=30,  # generous timeout so we see failures, not client-side cuts
        )


class AsyncUser(HttpUser):
    """
    Simulates a customer using the ASYNCHRONOUS endpoint.
    Each request returns immediately (202 Accepted).
    """
    wait_time = between(0.1, 0.5)

    @task
    def place_async_order(self):
        self.client.post(
            "/orders/async",
            json=make_order(),
            headers={"Content-Type": "application/json"},
            timeout=10,
        )