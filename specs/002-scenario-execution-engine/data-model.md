# Data Models for `002-scenario-execution-engine`

This feature does not introduce new data models. It extends the existing `Scenario` entity by defining the lifecycle of its `status` field during execution.

## 1. Scenario Status Lifecycle

The `status` field of the `Scenario` entity will transition through the following states during execution:

| Status | Description | Trigger |
|---|---|---|
| `created` | The initial state of a scenario after being successfully created. | `POST /api/v1/scenarios` |
| `running` | The scenario is actively being executed by the engine. | `POST /api/v1/scenarios/{id}/_execute` |
| `completed` | The scenario execution finished successfully. | The execution engine completes the test duration. |
| `failed` | The scenario execution was terminated due to an error. | An unrecoverable error occurs during execution, or the process is terminated. |
