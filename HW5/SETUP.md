# HW5 — Product API with AWS Deployment & Load Testing

## Overview

A RESTful Product API built with Python/Flask, containerized with Docker, deployed to AWS ECS Fargate using Terraform, and stress-tested with Locust. This project implements the Product service portion of the e-commerce OpenAPI 3.0.3 specification (`api.yaml`).

## Repository Structure

```
HW5/
├── app.py                  # Flask server — Product API endpoints
├── tests.py                # Pytest test suite (14 tests)
├── requirements.txt        # Python dependencies (flask, pytest)
├── Dockerfile              # Container definition (targets linux/amd64)
├── api.yaml                # OpenAPI specification (reference)
├── locust_http.py          # Load test using HttpUser
├── locust_fast.py          # Load test using FastHttpUser
├── README.md               # This file
│
└── CS6650_2b_demo/         # Forked Terraform infrastructure repo
    ├── src/                # Server code deployed to AWS
    │   ├── app.py
    │   ├── requirements.txt
    │   └── Dockerfile
    ├── terraform/          # Infrastructure as Code
    │   ├── main.tf         # Root module — wires together all modules
    │   ├── variables.tf    # Configurable variables (region, port, etc.)
    │   ├── outputs.tf      # Output values (cluster name, service name)
    │   ├── provider.tf     # AWS and Docker provider configuration
    │   └── modules/
    │       ├── ecr/        # Elastic Container Registry (Docker image store)
    │       ├── ecs/        # Elastic Container Service (Fargate tasks)
    │       ├── logging/    # CloudWatch log group
    │       └── network/    # VPC, subnets, security groups
    └── README.MD
```

## Prerequisites

Before deploying, make sure you have the following installed:

- **Python 3.10+** — [python.org](https://www.python.org/downloads/)
- **Docker Desktop** — [docker.com](https://www.docker.com/products/docker-desktop/)
- **AWS CLI v2** — [AWS install guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- **Terraform** — [terraform install guide](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
- **Locust** (for load testing) — `pip install locust`

---

## Part 1: Product API

### Product Schema

All fields are required:

| Field          | Type    | Constraints        | Example            |
|----------------|---------|--------------------|--------------------|
| `product_id`   | integer | minimum 1          | `12345`            |
| `sku`          | string  | 1–100 characters   | `"ABC-123-XYZ"`    |
| `manufacturer` | string  | 1–200 characters   | `"Acme Corporation"` |
| `category_id`  | integer | minimum 1          | `456`              |
| `weight`       | integer | minimum 0 (grams)  | `1250`             |
| `some_other_id`| integer | minimum 1          | `789`              |

### API Endpoints

| Method | Path                            | Description               | Success Code |
|--------|--------------------------------|---------------------------|--------------|
| GET    | `/products/{productId}`         | Retrieve a product by ID  | 200          |
| POST   | `/products/{productId}/details` | Add/update product details| 204          |

### Run Locally

```bash
pip install -r requirements.txt
python app.py
```

Server starts on `http://localhost:3000`.

### Run Tests

```bash
pip install -r requirements.txt
pytest tests.py -v
```

### API Examples with Response Codes

**Create a product — 204 No Content:**

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:3000/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "sku": "ABC-123-XYZ",
    "manufacturer": "Acme Corporation",
    "category_id": 456,
    "weight": 1250,
    "some_other_id": 789
  }'
# Response: 204 (no body)
```

**Get an existing product — 200 OK:**

```bash
curl http://localhost:3000/products/1
```
```json
{
  "category_id": 456,
  "manufacturer": "Acme Corporation",
  "product_id": 1,
  "sku": "ABC-123-XYZ",
  "some_other_id": 789,
  "weight": 1250
}
```

**Get a non-existent product — 404 Not Found:**

```bash
curl http://localhost:3000/products/9999
```
```json
{
  "error": "NOT_FOUND",
  "message": "Product not found",
  "details": "No product with ID 9999"
}
```

**Invalid input (missing fields) — 400 Bad Request:**

```bash
curl -X POST http://localhost:3000/products/1/details \
  -H "Content-Type: application/json" \
  -d '{"product_id": 1, "sku": "ABC-123-XYZ"}'
```
```json
{
  "error": "INVALID_INPUT",
  "message": "The provided input data is invalid",
  "details": "Missing required fields: category_id, manufacturer, some_other_id, weight"
}
```

**Mismatched product ID — 400 Bad Request:**

```bash
curl -X POST http://localhost:3000/products/1/details \
  -H "Content-Type: application/json" \
  -d '{"product_id": 999, "sku": "X", "manufacturer": "X", "category_id": 1, "weight": 0, "some_other_id": 1}'
```
```json
{
  "error": "INVALID_INPUT",
  "message": "The provided input data is invalid",
  "details": "product_id in body must match the productId in the URL path"
}
```

**Invalid JSON body — 400 Bad Request:**

```bash
curl -X POST http://localhost:3000/products/1/details \
  -H "Content-Type: application/json" \
  -d 'not json'
```
```json
{
  "error": "INVALID_INPUT",
  "message": "Request body must be valid JSON"
}
```

**Wrong HTTP method — 405 Method Not Allowed:**

```bash
curl -X DELETE http://localhost:3000/products/1
```
```json
{
  "error": "METHOD_NOT_ALLOWED",
  "message": "HTTP method not allowed for this endpoint"
}
```

### Error Response Format

All errors follow the standard schema from the OpenAPI spec:

```json
{
  "error": "ERROR_CODE",
  "message": "Human-readable description",
  "details": "Optional additional context"
}
```

| Status | Error Code          | When                                |
|--------|---------------------|-------------------------------------|
| 400    | INVALID_INPUT       | Bad request body or parameters      |
| 404    | NOT_FOUND           | Product does not exist              |
| 405    | METHOD_NOT_ALLOWED  | Wrong HTTP method for endpoint      |
| 500    | INTERNAL_ERROR      | Unexpected server error             |

### Data Structure Choice

Product data is stored in-memory using a Python dictionary (hashmap), keyed by `product_id`. This gives O(1) lookups for both reads and writes. In a real e-commerce system, reads (browsing/viewing products) vastly outnumber writes (creating/updating products), making a hashmap ideal for this read-heavy workload.

---

## Part 2: AWS Deployment with Terraform

### Architecture

```
Docker Image → ECR (registry) → ECS Fargate (runs container) → Public IP
                                        ↓
                                  CloudWatch Logs
```

Terraform manages four modules:
- **ECR** — stores the Docker image
- **ECS** — runs the container on Fargate (0.25 vCPU, 512MB RAM)
- **Network** — VPC, subnets, security group (opens port 3000)
- **Logging** — CloudWatch log group for container logs

### Deployment Instructions

**Step 1: Fork and clone the infrastructure repo**

```bash
# Fork https://github.com/RuidiH/CS6650_2b_demo on GitHub, then:
git clone https://github.com/<your-username>/CS6650_2b_demo.git
cd CS6650_2b_demo
```

**Step 2: Replace the Go server with the Product API**

```bash
rm src/main.go src/go.mod src/go.sum src/Dockerfile
cp ../app.py src/
cp ../requirements.txt src/
cp ../Dockerfile src/
```

**Step 3: Configure AWS credentials**

Get your temporary credentials from AWS Learner's Lab, then:

```bash
aws configure
# Access Key ID: <from Learner's Lab>
# Secret Access Key: <from Learner's Lab>
# Region: us-west-2
# Output format: json

aws configure set aws_session_token <YOUR-TEMP-SESSION-TOKEN>
```

**Step 4: Deploy with Terraform**

```bash
cd terraform
terraform init
terraform apply -auto-approve
```

This will build the Docker image, push it to ECR, and start the ECS Fargate task. Takes about 2–3 minutes.

**Step 5: Get the public IP**

```bash
aws ec2 describe-network-interfaces \
--network-interface-ids $(
    aws ecs describe-tasks \
    --cluster $(terraform output -raw ecs_cluster_name) \
    --tasks $(
        aws ecs list-tasks \
        --cluster $(terraform output -raw ecs_cluster_name) \
        --service-name $(terraform output -raw ecs_service_name) \
        --query 'taskArns[0]' --output text
    ) \
    --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
    --output text
) \
--query 'NetworkInterfaces[0].Association.PublicIp' \
--output text
```

**Step 6: Test your deployed API**

```bash
# Create a product
curl -X POST http://<PUBLIC-IP>:3000/products/1/details \
  -H "Content-Type: application/json" \
  -d '{"product_id":1,"sku":"ABC-123-XYZ","manufacturer":"Acme Corporation","category_id":456,"weight":1250,"some_other_id":789}'

# Retrieve it
curl http://<PUBLIC-IP>:3000/products/1
```

**Step 7: Check logs (if needed)**

```bash
aws logs tail /ecs/CS6650L2 --since 10m
```

### Important: Apple Silicon (M1/M2/M3) Macs

The Dockerfile must target `linux/amd64` since ECS Fargate runs on x86_64. The Dockerfile already includes `--platform=linux/amd64`. If Terraform's Docker build uses cached ARM layers, rebuild manually:

```bash
cd src
docker build --platform linux/amd64 --no-cache -t <ECR_URL>:latest .
aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin <ECR_URL>
docker push <ECR_URL>:latest
cd ../terraform
aws ecs update-service --cluster CS6650L2-cluster --service CS6650L2 --force-new-deployment
```

### Clean Up

```bash
cd terraform
terraform destroy -auto-approve
```

### .gitignore

Make sure the following are in your `.gitignore` to keep the repo clean:

```
# Terraform
*.tfstate
*.tfstate.backup
.terraform/
.terraform.lock.hcl
*.tfvars

# Secrets
.env
*.pem
*.key

# Python
__pycache__/
*.pyc
.pytest_cache/

# OS
.DS_Store
```

---

## Part 3: Load Testing with Locust

### Setup

```bash
pip install locust
```

### Test Files

- `locust_http.py` — uses `HttpUser` (Python `requests` library)
- `locust_fast.py` — uses `FastHttpUser` (`geventhttpclient` library)

Both files run identical workloads with a 4:1 read-to-write ratio to simulate real e-commerce traffic patterns.

### Running Tests

```bash
# HttpUser test
locust -f locust_http.py --host http://<PUBLIC-IP>:3000

# FastHttpUser test
locust -f locust_fast.py --host http://<PUBLIC-IP>:3000
```

Open `http://localhost:8089` in your browser to access the Locust UI.

### Test Scenarios

| Scenario       | Users | Spawn Rate | Duration |
|----------------|-------|------------|----------|
| Light load     | 10    | 2/s        | 1 min    |
| Medium load    | 50    | 10/s       | 2 min    |
| Stress test    | 200   | 20/s       | 2 min    |
| Breaking point | 500   | 50/s       | 2 min    |

### Results Summary

**HttpUser at 200 users (stress test):**
- ~211 RPS, 0 failures
- Median: 660ms, 95th percentile: 830ms, 99th: 1300ms

**FastHttpUser at 200 users (stress test):**
- ~210 RPS, 0 failures
- Median: 200ms, 95th percentile: 250ms
- Noticeably lower latency due to lighter client overhead

**Breaking point at 500 users:**
- RPS capped at ~215 — server ceiling reached
- Average response time spiked to ~2000ms
- 99th percentile hit 12,000ms (timeouts)
- 35 failures from connection timeouts
- Max response: 60,000ms (60s timeout)

### HttpUser vs FastHttpUser Analysis

At low concurrency (10–50 users), both perform similarly because network latency to AWS dominates — the client HTTP library overhead is negligible compared to round-trip time. At high concurrency (200+ users), FastHttpUser shows lower measured latency because `geventhttpclient` has less per-request CPU overhead than Python's `requests` library, allowing the client machine to process responses faster. The server-side throughput (RPS) remains the same since the bottleneck is the server, not the client.

---

## Using Postman

For easier manual testing, you can import the `api.yaml` OpenAPI spec directly into [Postman](https://www.postman.com/) (File → Import) to auto-generate a request collection. Alternatively, create a collection manually with the curl examples above.