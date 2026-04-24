from flowforge.models import WorkflowState


def test_workflow_state_records_events_and_current_step() -> None:
    state = WorkflowState(order_id="ord_123", status="running")

    state.record_event("check_inventory", "started", "checking inventory")
    state.record_event("check_inventory", "completed", "inventory ok")

    assert state.current_step == "check_inventory"
    assert len(state.events) == 2
    assert state.events[0].status == "started"
    assert state.events[1].status == "completed"
    assert state.steps_completed == []
