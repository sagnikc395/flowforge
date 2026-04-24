from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable


class InventoryStore:
    def __init__(self) -> None:
        self._lock = asyncio.Lock()
        self._stock = {
            "SKU-001": 10,
            "SKU-002": 5,
            "SKU-003": 20,
        }

    async def check_stock(self, product_id: str, quantity: int) -> None:
        async with self._lock:
            available = self._stock.get(product_id, 0)
            if available < quantity:
                raise ValueError(
                    f"Insufficient stock for {product_id}. Requested={quantity}, available={available}"
                )

    async def reserve_stock(self, product_id: str, quantity: int) -> None:
        async with self._lock:
            available = self._stock.get(product_id, 0)
            if available < quantity:
                raise ValueError(
                    f"Insufficient stock for {product_id}. Requested={quantity}, available={available}"
                )
            self._stock[product_id] = available - quantity

    async def release_stock(self, product_id: str, quantity: int) -> None:
        async with self._lock:
            self._stock[product_id] = self._stock.get(product_id, 0) + quantity


class PaymentStore:
    def __init__(self) -> None:
        self._lock = asyncio.Lock()
        self._charges_by_key: dict[str, str] = {}
        self._charge_status: dict[str, str] = {}

    async def charge(
        self, amount: int, payment_method: str, idempotency_key: str
    ) -> str:
        del amount, payment_method
        async with self._lock:
            existing = self._charges_by_key.get(idempotency_key)
            if existing is not None:
                return existing
            charge_id = f"ch_{idempotency_key.replace('-', '_')}"
            self._charges_by_key[idempotency_key] = charge_id
            self._charge_status[charge_id] = "charged"
            return charge_id

    async def refund(self, charge_id: str) -> None:
        async with self._lock:
            if charge_id in self._charge_status:
                self._charge_status[charge_id] = "refunded"


class WarehouseStore:
    def __init__(self) -> None:
        self._lock = asyncio.Lock()
        self._orders: dict[str, dict[str, object]] = {}

    async def update(self, order_id: str, product_id: str, quantity: int) -> None:
        async with self._lock:
            self._orders[order_id] = {
                "product_id": product_id,
                "quantity": quantity,
                "status": "reserved",
            }

    async def revert(self, order_id: str) -> None:
        async with self._lock:
            self._orders.pop(order_id, None)


inventory_store = InventoryStore()
payment_store = PaymentStore()
warehouse_store = WarehouseStore()

CompensationCallable = Callable[[], Awaitable[None]]
