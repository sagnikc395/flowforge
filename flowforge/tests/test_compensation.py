from flowforge.workflows.compensation import SagaCompensator


def test_compensator_registers_actions() -> None:
    compensator = SagaCompensator()
    compensator.add("release_inventory", "SKU-001", 2)
    assert len(compensator._actions) == 1
