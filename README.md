# flowforge
an "unbreakable" distributed workflow engine 


## Why This Exists
Traditional request/response APIs fall apart for multi-step processes. What happens when:

Payment succeeds but the warehouse update times out?
The payment processor goes down mid-transaction?
A partial failure leaves your data in an inconsistent state?

flowforge solves this using the Saga Pattern with Temporal.io — each step is durable, every failure is compensated, and the system self-heals.

 
## Tech Stack
 
| Layer | Technology | Purpose |
|---|---|---|
| API | FastAPI + Uvicorn | Order intake, async HTTP |
| Orchestration | Temporal.io (Python SDK) | Durable workflow execution |
| Payments | Stripe Mock | Payment processing simulation |
| Database | PostgreSQL | Warehouse inventory state |
| Async Runtime | Python AsyncIO | Non-blocking activity execution |
| Containerization | Docker + Docker Compose | Local dev environment |
 
 
## Project Structure
 
```
flowforge/
├── api/
│   ├── main.py              # FastAPI app — order endpoints
│   └── schemas.py           # Pydantic request/response models
├── workflows/
│   ├── fulfillment.py       # Core Temporal workflow definition
│   └── compensation.py      # Saga compensation logic
├── activities/
│   ├── inventory.py         # Check & update stock (Activity)
│   ├── payment.py           # Stripe mock (Activity)
│   └── warehouse.py         # DB warehouse update (Activity)
├── worker/
│   └── worker.py            # Temporal worker entrypoint
├── mocks/
│   ├── stripe_mock.py       # Fake Stripe API
│   └── inventory_api.py     # Fake external inventory service
├── db/
│   └── models.py            # SQLAlchemy models
├── tests/
│   ├── test_workflow.py     # Temporal workflow unit tests
│   └── test_compensation.py # Saga rollback tests
├── docker-compose.yml
├── Dockerfile
└── pyproject.toml
```
 
 
## The Saga Pattern — How Compensation Works
 
Each activity registers a **compensating action** before it executes. If any downstream step fails, Temporal unwinds the saga in reverse order.


## Getting Started
 
### Prerequisites
 
- Python 3.11+
- Docker & Docker Compose
- [Temporal CLI](https://docs.temporal.io/cli) (for local dev server)
### 1. Clone & Install
 
```bash
git clone https://github.com/yourusername/flowforge.git
cd flowforge
python -m venv venv && source venv/bin/activate
pip install -r requirements.txt
```
 
### 2. Start Temporal Server (local)
 
```bash
temporal server start-dev
```
 
This spins up a local Temporal server with a Web UI at `http://localhost:8233`.
 
### 3. Start the Worker
 
```bash
python worker/worker.py
```
 
The worker polls Temporal for workflow and activity tasks.
 
### 4. Start the FastAPI Server
 
```bash
uvicorn api.main:app --reload --port 8000
```
 
### 5. Place an Order
 
```bash
curl -X POST http://localhost:8000/orders \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "SKU-001",
    "quantity": 2,
    "customer_id": "cust-42",
    "payment_method": "tok_visa"
  }'
```
 
### 6. Simulate a Failure
 
Set the `FAIL_AT` environment variable to trigger compensation:
 
```bash
FAIL_AT=warehouse python worker/worker.py
```
 
Watch Temporal automatically refund the payment and re-stock inventory in the Web UI at `http://localhost:8233`.
 
 
## API Reference
 
### `POST /orders`
 
Place a new order and trigger the fulfillment workflow.
 
**Request Body**
```json
{
  "product_id": "string",
  "quantity": 1,
  "customer_id": "string",
  "payment_method": "string"
}
```
 
**Response**
```json
{
  "workflow_id": "order-uuid-1234",
  "status": "started",
  "order_id": "ord_abc123"
}
```
 
### `GET /orders/{workflow_id}/status`
 
Poll the current status of a running or completed workflow.
 
**Response**
```json
{
  "workflow_id": "order-uuid-1234",
  "status": "completed | running | compensating | failed",
  "steps_completed": ["check_stock", "process_payment", "update_warehouse"],
  "compensation_triggered": false
}
```
 
## Main Concepts Demonstrated
 
**Durable Execution** — Temporal persists every workflow step. If the worker process crashes and restarts, the workflow resumes exactly where it left off.
 
**Idempotency** — Every activity is designed to be safely retried. Processing the same payment twice won't double-charge.
 
**Saga Compensation** — Failures don't leave the system in a broken state. Every side effect has a registered undo operation.
 
**Async Activities** — All I/O (external APIs, DB writes) runs as non-blocking async activities, keeping throughput high.
 
**Visibility** — Every workflow run, activity execution, and compensation event is visible in the Temporal Web UI with full event history.
 
 
## Running Tests
 
```bash
# Unit tests (no Temporal server needed)
pytest tests/ -v
 
# Integration tests (requires running Temporal server)
TEMPORAL_HOST=localhost:7233 pytest tests/integration/ -v
```
 
 
## Environment Variables
 
| Variable | Default | Description |
|---|---|---|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `TASK_QUEUE` | `fulfillment-queue` | Temporal task queue name |
| `DATABASE_URL` | `sqlite:///./flowforge.db` | SQLAlchemy DB connection string |
| `STRIPE_SECRET_KEY` | `sk_test_mock` | Stripe API key (mock in dev) |
| `FAIL_AT` | `""` | Inject failure at step for testing |
 
 
## Deployment
 
A `docker-compose.yml` is included for local full-stack deployment:
 
```bash
docker-compose up --build
```
 
This starts:
- Temporal server + Postgres (persistence)
- FlowForge API (`localhost:8000`)
- FlowForge Worker
- Temporal Web UI (`localhost:8233`)
For production, deploy the API and Worker as separate services. Both connect to a managed Temporal Cloud instance or self-hosted Temporal cluster.

## References
 
- [Temporal.io Python SDK Docs](https://docs.temporal.io/develop/python)
- [Saga Pattern — Microsoft Architecture Guide](https://learn.microsoft.com/en-us/azure/architecture/reference-architectures/saga/saga)
- [FastAPI Async Docs](https://fastapi.tiangolo.com/async/)
