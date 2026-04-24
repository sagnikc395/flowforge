from __future__ import annotations

import os

TEMPORAL_HOST = os.getenv("TEMPORAL_HOST", "localhost:7233")
TASK_QUEUE = os.getenv("TASK_QUEUE", "fulfillment-queue")
