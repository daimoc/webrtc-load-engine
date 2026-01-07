# Quickstart: Executing a Load Test Scenario

This guide explains how to use the `/api/v1/scenarios/{id}/_execute` endpoint to start the execution of a load test scenario.

## Prerequisites

- The WebRTC Load Engine server must be running.
- You must have already created a scenario and have its ID.
- You need a tool like `curl` or Postman to make API requests.

## Endpoint

-   **Method**: `POST`
-   **URL**: `http://localhost:8080/api/v1/scenarios/{id}/_execute`

Replace `{id}` with the ID of the scenario you want to execute.

## Example using `curl`

You can execute a scenario using the following `curl` command:

```bash
# Replace 'scenario-a1b2c3d4' with your scenario ID
curl -X POST http://localhost:8080/api/v1/scenarios/scenario-a1b2c3d4/_execute
```

## Success Response

A successful request will return a `202 Accepted` status code, indicating that the server has accepted the request and will start the execution asynchronously.

-   **Status**: `202 Accepted`
-   **Body**: *(empty)*

## Error Responses

-   **Status**: `404 Not Found`
    -   **Reason**: The scenario with the specified ID does not exist.
-   **Status**: `409 Conflict`
    -   **Reason**: The scenario is already in the `running` state.
