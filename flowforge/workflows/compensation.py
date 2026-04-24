from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass, field
from datetime import timedelta
from temporalio import workflow


@dataclass
class CompensationAction:
    activity_name: str
    args: Sequence[object] = field(default_factory=tuple)


class SagaCompensator:
    def __init__(self) -> None:
        self._actions: list[CompensationAction] = []

    def add(self, activity_name: str, *args: object) -> None:
        self._actions.append(CompensationAction(activity_name=activity_name, args=args))

    async def compensate(self) -> None:
        for action in reversed(self._actions):
            await workflow.execute_activity(
                action.activity_name,
                args=list(action.args),
                schedule_to_close_timeout=timedelta(seconds=10),
            )
