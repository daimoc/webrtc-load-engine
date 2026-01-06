# Quickstart: Creating a Load Test Scenario

This guide explains how to use the `/api/v1/scenarios` endpoint to create a new load test scenario.

## Prerequisites

-   The WebRTC Load Engine server must be running.
-   You need a tool like `curl` or Postman to make API requests.

## Endpoint

-   **Method**: `POST`
-   **URL**: `http://localhost:8080/api/v1/scenarios` (assuming the server runs on port 8080)

## Request Body

The request body must be a JSON object with the following structure:

```json
{
  "name": "My First Load Test",
  "platform": "jitsi",
  "test_plan": {
    "participants": 100,
    "ramp_up_seconds": 60,
    "duration_seconds": 300
  }
}
```

## Example using `curl`

You can create a scenario using the following `curl` command:

```bash
curl -X POST http://localhost:8080/api/v1/scenarios \
-H "Content-Type: application/json" \
-d 
{
  "name": "My First Load Test",
  "platform": "jitsi",
  "test_plan": {
    "participants": 100,
    "ramp_up_seconds": 60,
    "duration_seconds": 300
  }
}
```

## Success Response

A successful request will return a `201 Created` status code and a JSON body with the ID of the newly created scenario.

-   **Status**: `201 Created`
-   **Body**:

```json
{
  "id": "scenario-a1b2c3d4",
  "status": "created"
}
```

## Error Response

If the request is invalid (e.g., missing required fields, invalid values), the server will respond with a `400 Bad Request` status code and an error message.

-   **Status**: `400 Bad Request`
-   **Body**:

```json
{
  "error": "Invalid platform adapter: skype"
}
```
