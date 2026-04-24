from __future__ import annotations

from collections.abc import Callable
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
            await self._run_step(
                "check_inventory",
                order.product_id,
                order.quantity,
            )
            await self._run_step(
                "reserve_inventory",
                order.product_id,
                order.quantity,
                compensator=compensator,
                compensation_args=(order.product_id, order.quantity),
                compensation_name="release_inventory",
            )
            payment = await self._run_step(
                "process_payment",
                order.customer_id,
                amount,
                order.payment_method,
                order_id,
                compensator=compensator,
                compensation_name="refund_payment",
                compensation_arg_factory=lambda result: (result.charge_id,),
            )
            await self._run_step(
                "update_warehouse",
                order_id,
                order.product_id,
                order.quantity,
                compensator=compensator,
                compensation_args=(order_id,),
                compensation_name="revert_warehouse",
            )
            self.state.status = "completed"
            self.state.record_event("workflow", "completed", "Fulfillment completed")
            return self.state
        except Exception as exc:
            self.state.status = "compensating"
            self.state.compensation_triggered = True
            self.state.failure_reason = str(exc)
            self.state.record_event(
                self.state.current_step or "workflow",
                "failed",
                f"Workflow failed: {exc}",
            )
            await compensator.compensate(self.state)
            self.state.status = "failed"
            return self.state

    async def _activity(self, name: str, *args: object):
        return await workflow.execute_activity(
            name,
            args=list(args),
            schedule_to_close_timeout=ACTIVITY_TIMEOUT,
            retry_policy=ACTIVITY_RETRY_POLICY,
        )

    async def _run_step(
        self,
        step_name: str,
        *args: object,
        compensator: SagaCompensator | None = None,
        compensation_name: str | None = None,
        compensation_args: tuple[object, ...] | None = None,
        compensation_arg_factory: Callable[[object], tuple[object, ...]] | None = None,
    ):
        self.state.record_event(step_name, "started", f"{step_name} started")
        result = await self._activity(step_name, *args)
        self.state.steps_completed.append(step_name)
        self.state.record_event(step_name, "completed", f"{step_name} completed")
        if compensator is not None and compensation_name is not None:
            if compensation_arg_factory is not None:
                args_to_register = compensation_arg_factory(result)
            else:
                args_to_register = compensation_args or ()
            compensator.add(compensation_name, *args_to_register)
        return result
