# Quickstart: Jitsi XMPP Signaling

This guide shows how to configure the Load Engine to test a Jitsi Meet instance using the XMPP adapter.

## Prerequisites
- A running Jitsi Meet instance (public or private).
- Network access to the XMPP WebSocket (usually port 443 wss).

## Configuration

In your scenario definition (JSON), set the `platform` to `jitsi` and provide the `platform_config`:

```json
{
  "name": "Basic Jitsi Join",
  "platform": "jitsi",
  "test_plan": {
    "duration": "30s",
    "platform_config": {
      "xmpp_domain": "meet.jit.si",
      "room_name": "MyLoadTestRoom",
      "muc_domain": "conference.meet.jit.si",
      "debug": true
    }
  }
}
```

## Running the Test

```bash
# Start the engine
./webrtc-load-engine

# Trigger the scenario via API
curl -X POST http://localhost:8080/api/v1/scenarios/jitsi-scenario/_execute
```
