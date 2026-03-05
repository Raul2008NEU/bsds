from locust import task, constant
from locust.contrib.fasthttp import FastHttpUser
import random

SEARCH_TERMS = ["electronics", "books", "home", "alpha", "beta", "sports"]


class ProductSearchUser(FastHttpUser):
    wait_time = constant(0)

    @task(10)
    def search_products(self):
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
            f"/search?q={term}",
            catch_response=True,
            name="/search",
        ) as resp:
            if resp.status_code == 200:
                data = resp.json()
                reco_status = data.get("reco_status", "unknown")
                # Search succeeding is what matters
                # Recommendations failing is expected and handled
                if "products" not in data:
                    resp.failure("Missing products in response")
                else:
                    resp.success()
                    # Track reco status in logs
                    if reco_status not in ("ok", "circuit_open", "unavailable"):
                        print(f"Unexpected reco status: {reco_status}")
            else:
                resp.failure(f"HTTP {resp.status_code}")

    @task(1)
    def check_circuit(self):
        # Monitor the circuit breaker state during the test
        with self.client.get(
            "/circuit",
            catch_response=True,
            name="/circuit"
        ) as resp:
            if resp.status_code == 200:
                data = resp.json()
                state = data.get("circuit_state", "unknown")
                if state != "CLOSED":
                    print(f"⚡ Circuit breaker state: {state}")
                resp.success()
            else:
                resp.failure(f"HTTP {resp.status_code}")