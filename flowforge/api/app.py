from __future__ import annotations

import uuid

from fastapi import FastAPI, HTTPException
from temporalio.client import Client, WorkflowHandle
from temporalio.service import RPCError

from flowforge.config import TASK_QUEUE, TEMPORAL_HOST
from flowforge.api.schemas import OrderRequest, OrderResponse, WorkflowStatusResponse
from flowforge.models import (
    EngineSnapshot,
    InventorySnapshot,
    OrderSummary,
    PaymentRecordView,
    WarehouseRecordView,
)
from flowforge.store import (
    inventory_store,
    payment_store,
    warehouse_store,
    workflow_registry,
)
from flowforge.workflows.workflows import FulfillmentWorkflow

app = FastAPI(title="FlowForge")


async def get_temporal_client() -> Client:
    return await Client.connect(TEMPORAL_HOST)


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/orders", response_model=OrderResponse)
async def create_order(order: OrderRequest) -> OrderResponse:
    client = await get_temporal_client()
    order_id = f"ord_{uuid.uuid4().hex[:12]}"
    workflow_id = order.workflow_id or f"order-{order_id}"
    existing = await workflow_registry.get(workflow_id)
    if existing is not None:
        raise HTTPException(status_code=409, detail="Workflow ID already exists")
    await client.start_workflow(
        FulfillmentWorkflow.run,
        args=[order_id, order],
        id=workflow_id,
        task_queue=TASK_QUEUE,
    )
    await workflow_registry.record(workflow_id, order_id, "started")
    return OrderResponse(workflow_id=workflow_id, order_id=order_id, status="started")


@app.get("/orders", response_model=list[OrderSummary])
async def list_orders() -> list[OrderSummary]:
    return await workflow_registry.list()


@app.get("/orders/{workflow_id}/status", response_model=WorkflowStatusResponse)
async def get_order_status(workflow_id: str) -> WorkflowStatusResponse:
    client = await get_temporal_client()
    handle: WorkflowHandle = client.get_workflow_handle(workflow_id)
    try:
        state = await handle.query(FulfillmentWorkflow.get_status)
    except RPCError as exc:
        raise HTTPException(status_code=404, detail="Workflow not found") from exc

    await workflow_registry.record(workflow_id, state.order_id, state.status)
    return WorkflowStatusResponse(workflow_id=workflow_id, **state.model_dump())


@app.get("/inventory", response_model=list[InventorySnapshot])
async def get_inventory() -> list[InventorySnapshot]:
    return await inventory_store.snapshot()


@app.get("/payments/{charge_id}", response_model=PaymentRecordView)
async def get_payment(charge_id: str) -> PaymentRecordView:
    payment = await payment_store.get(charge_id)
    if payment is None:
        raise HTTPException(status_code=404, detail="Payment not found")
    return payment


@app.get("/warehouse", response_model=list[WarehouseRecordView])
async def get_warehouse() -> list[WarehouseRecordView]:
    return await warehouse_store.snapshot()


@app.get("/engine/snapshot", response_model=EngineSnapshot)
async def get_engine_snapshot() -> EngineSnapshot:
    return EngineSnapshot(
        orders=await workflow_registry.list(),
        inventory=await inventory_store.snapshot(),
        payments=await payment_store.snapshot(),
        warehouse=await warehouse_store.snapshot(),
    )
