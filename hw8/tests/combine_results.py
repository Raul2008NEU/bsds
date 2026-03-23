#!/usr/bin/env python3
"""
Combine MySQL and DynamoDB test results into combined_results.json
and print the full comparison analysis for Step III.
"""

import json
import statistics

# Load both result files
with open("../results/mysql_test_results.json") as f:
    mysql_data = json.load(f)
with open("../results/dynamodb_test_results.json") as f:
    dynamo_data = json.load(f)

# Tag each record with its database
for r in mysql_data:
    r["database"] = "mysql"
for r in dynamo_data:
    r["database"] = "dynamodb"

combined = mysql_data + dynamo_data

with open("../results/combined_results.json", "w") as f:
    json.dump(combined, f, indent=2)

print(f"combined_results.json written: {len(combined)} total records")
print(f"  MySQL:    {len(mysql_data)} ops")
print(f"  DynamoDB: {len(dynamo_data)} ops")

# --- Helper functions ---
def get_times(data, op=None):
    if op:
        return [r["response_time"] for r in data if r["operation"] == op]
    return [r["response_time"] for r in data]

def percentile(times, p):
    s = sorted(times)
    k = (len(s) - 1) * (p / 100)
    f = int(k)
    c = f + 1
    if c >= len(s):
        return s[f]
    return s[f] + (k - f) * (s[c] - s[f])

def success_rate(data):
    return sum(1 for r in data if r["success"]) / len(data) * 100

# --- Part 1: Performance Comparison Table ---
print("\n" + "=" * 70)
print("PART 1: PERFORMANCE COMPARISON TABLE")
print("=" * 70)

mysql_times = get_times(mysql_data)
dynamo_times = get_times(dynamo_data)

metrics = [
    ("Avg Response Time (ms)", statistics.mean, None),
    ("P50 Response Time (ms)", lambda t: percentile(t, 50), None),
    ("P95 Response Time (ms)", lambda t: percentile(t, 95), None),
    ("P99 Response Time (ms)", lambda t: percentile(t, 99), None),
]

print(f"\n{'Metric':<25} {'MySQL':>10} {'DynamoDB':>10} {'Winner':>10} {'Margin':>10}")
print("-" * 70)

for name, func, _ in metrics:
    m_val = func(mysql_times)
    d_val = func(dynamo_times)
    winner = "MySQL" if m_val < d_val else "DynamoDB"
    margin = f"{abs(m_val - d_val):.2f} ms"
    print(f"{name:<25} {m_val:>10.2f} {d_val:>10.2f} {winner:>10} {margin:>10}")

m_sr = success_rate(mysql_data)
d_sr = success_rate(dynamo_data)
sr_winner = "Tie" if m_sr == d_sr else ("MySQL" if m_sr > d_sr else "DynamoDB")
print(f"{'Success Rate (%)':<25} {m_sr:>10.1f} {d_sr:>10.1f} {sr_winner:>10} {abs(m_sr-d_sr):.1f}%")
print(f"{'Total Operations':<25} {len(mysql_data):>10} {len(dynamo_data):>10}")

# --- Operation-Specific Breakdown ---
print(f"\n{'Operation':<15} {'MySQL Avg':>12} {'DynamoDB Avg':>14} {'Faster By':>12}")
print("-" * 55)

for op in ["create_cart", "add_items", "get_cart"]:
    m_avg = statistics.mean(get_times(mysql_data, op))
    d_avg = statistics.mean(get_times(dynamo_data, op))
    diff = abs(m_avg - d_avg)
    faster = "MySQL" if m_avg < d_avg else "DynamoDB"
    print(f"{op:<15} {m_avg:>10.2f}ms {d_avg:>12.2f}ms {faster} {diff:.2f}ms")

print("\n" + "=" * 70)
print("Data source: combined_results.json")
print("=" * 70)