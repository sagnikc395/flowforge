from __future__ import annotations

from pydantic import BaseModel, Field


class OrderRequest(BaseModel):
    product_id: str = Field(..., min_length=1)
    quantity: int = Field(..., gt=0)
    customer_id: str = Field(..., min_length=1)
    payment_method: str = Field(..., min_length=1)


class OrderResponse(BaseModel):
    workflow_id: str
    order_id: str
    status: str


class WorkflowStatusResponse(BaseModel):
    workflow_id: str
    order_id: str
    status: str
    steps_completed: list[str]
    compensation_triggered: bool
    failure_reason: str | None = None


class InventoryReservation(BaseModel):
    product_id: str
    quantity: int


class PaymentResult(BaseModel):
    charge_id: str
    status: str = "charged"


class WarehouseUpdate(BaseModel):
    order_id: str
    product_id: str
    quantity: int


class WorkflowState(BaseModel):
    order_id: str
    status: str
    steps_completed: list[str] = Field(default_factory=list)
    compensation_triggered: bool = False
    failure_reason: str | None = None
