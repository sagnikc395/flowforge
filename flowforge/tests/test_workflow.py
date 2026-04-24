from flowforge.models import WorkflowState


def test_workflow_state_defaults() -> None:
    state = WorkflowState(order_id="ord_123", status="running")
    assert state.steps_completed == []
    assert state.compensation_triggered is False
