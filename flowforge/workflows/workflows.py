from __future__ import annotations

from datetime import timedelta

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from flowforge.models import OrderRequest, WorkflowState
    from flowforge.workflows.compensation import SagaCompensator


ACTIVITY_TIMEOUT = timedelta(seconds=15)
ACTIVITY_RETRY_POLICY = RetryPolicy(
    initial_interval=timedelta(seconds=1),
    backoff_coefficient=2.0,
    maximum_interval=timedelta(seconds=5),
    maximum_attempts=3,
)


@workflow.defn
class FulfillmentWorkflow:
    def __init__(self) -> None:
        self.state = WorkflowState(order_id="", status="pending")

    @workflow.query
    def get_status(self) -> WorkflowState:
        return self.state

    @workflow.run
    async def run(self, order_id: str, order: OrderRequest) -> WorkflowState:
        self.state = WorkflowState(order_id=order_id, status="running")
        compensator = SagaCompensator()
        amount = order.quantity * 1000

        try:
            await self._activity("check_inventory", order.product_id, order.quantity)
            self.state.steps_completed.append("check_inventory")

            await self._activity("reserve_inventory", order.product_id, order.quantity)
            compensator.add("release_inventory", order.product_id, order.quantity)
            self.state.steps_completed.append("reserve_inventory")

            payment = await self._activity(
                "process_payment",
                order.customer_id,
                amount,
                order.payment_method,
                order_id,
            )
            compensator.add("refund_payment", payment.charge_id)
            self.state.steps_completed.append("process_payment")

            await self._activity(
                "update_warehouse", order_id, order.product_id, order.quantity
            )
            compensator.add("revert_warehouse", order_id)
            self.state.steps_completed.append("update_warehouse")

            self.state.status = "completed"
            return self.state
        except Exception as exc:
            self.state.status = "compensating"
            self.state.compensation_triggered = True
            self.state.failure_reason = str(exc)
            await compensator.compensate()
            self.state.status = "failed"
            return self.state

    async def _activity(self, name: str, *args: object):
        return await workflow.execute_activity(
            name,
            args=list(args),
            schedule_to_close_timeout=ACTIVITY_TIMEOUT,
            retry_policy=ACTIVITY_RETRY_POLICY,
        )
