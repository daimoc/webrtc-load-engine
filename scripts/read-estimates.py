#!/usr/bin/env python3
"""Read the media server's bandwidth estimate toward each receiver, once per
interval, for the 015 comparison runs (specs/015-bandwidth-estimate-parity,
research R7). Validation tooling: the engine does not use it.

    python3 -I scripts/read-estimates.py jitsi [--interval 1] > estimates/jitsi-vp8-1.jsonl

One JSON object per receiver per reading on stdout:

    {"t": "<UTC>", "server": "jitsi", "receiver": "<endpoint id>|browser",
     "endpoint": "<id>", "estimate_bps": N, "ideal_bps": N, "target_bps": N,
     "suspended": bool, "method": "jvb-debug"}

estimate_bps is the bridge's bandwidth estimate toward the receiver
(bitrate_allocator.bweBps); ideal_bps what it would send the receiver with
no limit, that is what its streams need (allocation.idealBps); target_bps
what it allocates within the estimate; suspended whether it holds back some
of the receiver's streams. A bot's endpoint id is the first 8 hex digits of
its JID's local part; the browser is the endpoint with a stats_id (bots have
none).

JVB_DEBUG_URL (default http://127.0.0.1:8080) is the bridge's private HTTP
interface, published on loopback by tests/platforms/docker-jitsi-meet.
Stops on SIGINT or SIGTERM. Standard library only.

mediasoup gives no per-receiver estimate without changing its code (R12
fact 4): its comparison uses the layer forwarded over time instead.
"""

import argparse
import datetime
import json
import os
import signal
import sys
import time
import urllib.request


def fetch(url):
    with urllib.request.urlopen(url, timeout=2) as r:
        return json.load(r)


def endpoint_lines(state, now):
    """The lines of one conference's full debug state."""
    for conf in (state.get("conferences") or {}).values():
        for eid, e in (conf.get("endpoints") or {}).items():
            a = (e.get("bitrate_controller") or {}).get("bitrate_allocator") or {}
            alloc = a.get("allocation") or {}
            yield {
                "t": now,
                "server": "jitsi",
                "receiver": "browser" if e.get("stats_id") else eid,
                "endpoint": eid,
                "estimate_bps": a.get("bweBps"),
                "ideal_bps": alloc.get("idealBps"),
                "target_bps": alloc.get("targetBps"),
                "suspended": alloc.get("has_suspended_sources"),
                "method": "jvb-debug",
            }


def main():
    p = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    p.add_argument("server", choices=["jitsi", "mediasoup"])
    p.add_argument("--interval", type=float, default=1.0)
    args = p.parse_args()
    if args.server == "mediasoup":
        sys.exit("mediasoup gives no per-receiver estimate to read (015 research R12, fact 4); "
                 "compare the layers forwarded over time instead")
    base = os.environ.get("JVB_DEBUG_URL", "http://127.0.0.1:8080")
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))
    try:
        while True:
            start = time.monotonic()
            now = datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")
            try:
                for conf in (fetch(base + "/debug").get("conferences") or {}):
                    for line in endpoint_lines(fetch(f"{base}/debug/{conf}?full=true"), now):
                        print(json.dumps(line), flush=True)
            except (OSError, ValueError) as e:
                print(f"read-estimates: {e}", file=sys.stderr)
            time.sleep(max(0.0, args.interval - (time.monotonic() - start)))
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
