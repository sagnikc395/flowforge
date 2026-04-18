# entry point for the execute the workflow

import asyncio
import uuid
from temporalio.client import Client


async def main():
    client = await Client.connect("localhost:7233")
    result = await client.execute_workflow(
        "SayHelloWorkflow",
        "Temporal",
        id=f"say-hello-workflow-{uuid.uuid4()}",
        task_queue="my-task-queue",
        result_type=str,
    )

    print(f"Workflow result: {result}")


if __name__ == "__main__":
    asyncio.run(main())
