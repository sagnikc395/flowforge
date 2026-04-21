# FlowForge — Distributed Workflow Engine

## What This Project Is

FlowForge is a backend system that demonstrates how to build fault-tolerant, multi-step distributed workflows using **Temporal.io** as the orchestration engine and **FastAPI** as the HTTP layer.

The core problem it solves: traditional request/response APIs are not designed for multi-step processes that span external services. If a three-step process — check inventory, charge payment, update warehouse — fails at step three, the customer has been charged and stock has been reserved, but the order was never confirmed. The system is now in an inconsistent state with no automatic recovery path.

FlowForge addresses this by modeling the entire order fulfillment process as a **durable Temporal workflow**. Every step is persisted. Every failure triggers a compensating rollback (the Saga Pattern). The system self-heals.

---

## The Problem It Solves

Consider a standard e-commerce order:

1. Check that the item is in stock
2. Charge the customer's payment method
3. Update the warehouse to reflect the reserved stock
4. Confirm the order

Without durable orchestration, any of the following can silently corrupt your data:

- The payment succeeds but the warehouse service times out
- The worker process crashes between steps two and three
- A network partition causes a partial write to the database
- A downstream service returns a transient 500 that retries indefinitely

Handling all of these failure modes correctly with manual retry logic, distributed locks, and rollback code is difficult, brittle, and easy to get wrong.

**Temporal.io** makes this manageable by treating the workflow as a persistent, resumable program. If the worker crashes mid-execution, Temporal replays the event history and continues exactly where it left off. Activities are retried automatically with configurable backoff. Compensating actions (refunds, restocks) are triggered deterministically when a failure propagates far enough to require rollback.

---

## How It Works

### Architecture Overview

```
HTTP Client
    │
    ▼
FastAPI (api/app.py)          ← receives order requests, triggers workflows
    │
    ▼
Temporal Client               ← submits workflow execution to Temporal server
    │
    ▼
Temporal Server               ← persists workflow state, schedules tasks
    │
    ▼
Temporal Worker (worker/worker.py)   ← polls for tasks, executes activities
    │
    ├── check_inventory()     ← activity: verify stock availability
    ├── process_payment()     ← activity: charge via Stripe (mock)
    └── update_warehouse()    ← activity: commit stock reservation to DB
```

### Workflow Execution Lifecycle

1. A client sends a `POST /orders` request to the FastAPI server with a product ID, quantity, customer ID, and payment method.
2. The API uses the Temporal client to start a `FulfillmentWorkflow` execution, returning a `workflow_id` immediately. The API does not wait for completion.
3. The Temporal worker picks up the workflow task and begins executing activities in sequence.
4. Each activity is atomic and idempotent — safe to retry without side effects (e.g., charging the same payment twice is guarded against).
5. If all activities succeed, the workflow completes and the order is confirmed.
6. If any activity fails after exhausting its retry policy, the workflow triggers **Saga compensation**: previously completed steps are undone in reverse order (warehouse unreserved → payment refunded → stock released).
7. The client can poll `GET /orders/{workflow_id}/status` to observe the current state at any point.

### The Saga Pattern

Each activity that produces a side effect registers a corresponding compensating action before it executes:

| Activity | Compensating Action |
|---|---|
| `check_inventory` | Release the reserved stock |
| `process_payment` | Issue a refund |
| `update_warehouse` | Revert the warehouse record |

If the workflow fails at `update_warehouse`, Temporal runs `refund_payment` and `release_inventory` in reverse order. The system returns to a consistent state without manual intervention.

---

## Tech Stack

| Layer | Technology | Role |
|---|---|---|
| API | FastAPI + Uvicorn | HTTP interface for order intake |
| Workflow Orchestration | Temporal.io (Python SDK) | Durable, resumable workflow execution |
| Activities | Python AsyncIO | Non-blocking I/O for each workflow step |
| Payment | Stripe Mock | Simulated payment processing |
| Database | PostgreSQL | Warehouse inventory persistence |
| Containerization | Docker + Docker Compose | Local full-stack development environment |

---

## Current Implementation State

The project is in early development. The scaffold is in place and the Temporal integration is wired end-to-end with a working proof-of-concept:

- `api/app.py` — FastAPI app with a `/health` endpoint
- `workflows/workflows.py` — `SayHelloWorkflow` wired to Temporal, demonstrates the execution model
- `activities/activities.py` — `greet` activity, demonstrates the activity definition pattern
- `worker/worker.py` — Temporal worker that registers workflows and activities and polls for tasks
- `api/starter.py` — CLI entrypoint that triggers a workflow execution directly via the Temporal client

The full fulfillment workflow (inventory, payment, warehouse, compensation logic) is the next implementation target.

---

## Key Concepts Demonstrated

**Durable Execution** — Temporal persists every workflow step as an event in its history. A worker crash does not lose progress; the workflow resumes from the last confirmed state on restart.

**Idempotency** — Every activity is designed to be safely retried. Charging the same payment intent twice will not double-charge the customer.

**Saga Compensation** — Failures do not leave the system in a broken intermediate state. Every side-effecting activity has a registered undo operation that Temporal executes automatically on failure propagation.

**Async Activities** — All I/O (external API calls, database writes) runs as non-blocking async activities, keeping worker throughput high under concurrency.

**Observability** — Every workflow run, activity execution, retry, and compensation event is recorded in Temporal's event history and visible in the Temporal Web UI at `localhost:8233`.

---

## Running Locally

```bash
# 1. Start Temporal server
temporal server start-dev

# 2. Start the worker
python -m flowforge.worker.worker

# 3. Start the API
uvicorn flowforge.api.app:app --reload --port 8000

# 4. Trigger a workflow
python -m flowforge.api.starter
```

To simulate a failure and observe Saga compensation:

```bash
FAIL_AT=warehouse python -m flowforge.worker.worker
```

Watch the Temporal Web UI at `http://localhost:8233` to see the compensation activities execute in reverse order.
