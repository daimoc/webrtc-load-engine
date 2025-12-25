# API Endpoint Specification: Create Load Test Scenario

## 1. Objective

-   **What is the goal of this endpoint?** To allow users to define and create a new load test scenario.
-   **Why is it needed?** This is the core functionality of the orchestration layer, enabling users to configure and run load tests.

## 2. Behavior

### 2.1. Happy Path

-   **Scenario:** A user submits a valid scenario definition.
-   **Given:** The user provides a name, a valid platform adapter (e.g., "jitsi"), and a valid test plan (e.g., number of participants, ramp-up time).
-   **When:** The user sends a POST request to `/api/v1/scenarios`.
-   **Then:** The system should respond with a 201 Created status, confirming the scenario has been created, and return the ID of the new scenario.

### 2.2. Edge Cases

-   **Scenario:** A user submits a scenario with an unsupported platform adapter.
-   **Given:** The user provides an adapter name that is not implemented (e.g., "skype").
-   **When:** The user sends a POST request to `/api/v1/scenarios`.
-   **Then:** The system should respond with a 400 Bad Request status and an error message indicating the platform is not supported.

-   **Scenario:** A user submits a scenario with an invalid test plan (e.g., negative number of participants).
-   **Given:** The user provides a test plan with invalid parameters.
-   **When:** The user sends a POST request to `/api/v1/scenarios`.
-   **Then:** The system should respond with a 400 Bad Request status and a descriptive error message.

## 3. Technical Details

-   **Method:** `POST`
-   **Endpoint:** `/api/v1/scenarios`
-   **Request Body:**
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
-   **Success Response:**
    -   **Status:** `201 Created`
    -   **Body:**
        ```json
        {
          "id": "scenario-_SCENARIO_ID_",
          "status": "created"
        }
        ```
-   **Error Response:**
    -   **Status:** `400 Bad Request`
    -   **Body:**
        ```json
        {
          "error": "Invalid platform adapter: skype"
        }
        ```
