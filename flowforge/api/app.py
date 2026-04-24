from __future__ import annotations

import uuid

from fastapi import FastAPI, HTTPException
from temporalio.client import Client, WorkflowHandle
from temporalio.service import RPCError

from flowforge.config import TASK_QUEUE, TEMPORAL_HOST
from flowforge.api.schemas import OrderRequest, OrderResponse, WorkflowStatusResponse
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
    workflow_id = f"order-{order_id}"
    await client.start_workflow(
        FulfillmentWorkflow.run,
        args=[order_id, order],
        id=workflow_id,
        task_queue=TASK_QUEUE,
    )
    return OrderResponse(workflow_id=workflow_id, order_id=order_id, status="started")


@app.get("/orders/{workflow_id}/status", response_model=WorkflowStatusResponse)
async def get_order_status(workflow_id: str) -> WorkflowStatusResponse:
    client = await get_temporal_client()
    handle: WorkflowHandle = client.get_workflow_handle(workflow_id)
    try:
        state = await handle.query(FulfillmentWorkflow.get_status)
    except RPCError as exc:
        raise HTTPException(status_code=404, detail="Workflow not found") from exc

    return WorkflowStatusResponse(workflow_id=workflow_id, **state.model_dump())
