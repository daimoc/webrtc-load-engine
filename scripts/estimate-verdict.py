#!/usr/bin/env python3
"""Per-receiver table for a 015 comparison run (research R11): when each
receiver reached what its streams need, and how long each bot stayed under
it after the browser reached its own. Validation tooling only.

Jitsi, from the bridge's estimates (scripts/read-estimates.py):

    python3 -I scripts/estimate-verdict.py jitsi estimates/jitsi-vp8-1.jsonl

  need = the bridge's ideal_bps for the receiver; reached = the first reading
  with estimate_bps >= ideal_bps and no suspended stream. Readings of -1
  (no estimate yet for a new endpoint) are left out.

mediasoup, from the layer forwarded over time (it gives no estimate, R12
fact 4): the run's JSON report (the browser's per-second heights) and the
live test's log (the bots' "Received video height changed" lines):

    python3 -I scripts/estimate-verdict.py mediasoup report.json test.log

  reached = every stream the receiver decodes is at the height it asked for
  (the browser: its expected_height).

A bot is "held low" when it is under its need for more than 1 s (FR-008's
margin) after the browser reached its own, or falls back under it later.
Prints a Markdown table. --self-test runs the checks on scripts/testdata.
"""

import datetime
import json
import os
import re
import sys

MARGIN = 1.0  # Seconds after the browser (FR-008)


def parse_time(s):
    return datetime.datetime.fromisoformat(s.replace("Z", "+00:00"))


class Track:
    """A receiver's 'at its need' state over time, as (time, ok) changes."""

    def __init__(self, name):
        self.name, self.changes, self.first, self.last = name, [], None, None

    def add(self, t, ok):
        if self.first is None:
            self.first = t
        self.last = t
        if not self.changes or self.changes[-1][1] != ok:
            self.changes.append((t, ok))

    def reached(self):
        return next((t for t, ok in self.changes if ok), None)

    def under_after(self, start, end):
        """Seconds not at need between start and end."""
        total, state, since = 0.0, None, start
        for t, ok in self.changes:
            if t <= start:
                state = ok
                continue
            if t >= end:
                break
            if state is False:
                total += (t - since).total_seconds()
            state, since = ok, t
        if state is False:
            total += (end - since).total_seconds()
        return total


def jitsi_tracks(lines):
    tracks = {}
    for line in lines:
        r = json.loads(line)
        # -1: the bridge has no estimate yet for a new endpoint.
        if r.get("estimate_bps") is None or r["estimate_bps"] < 0 or not r.get("ideal_bps"):
            continue
        t = parse_time(r["t"])
        ok = r["estimate_bps"] >= r["ideal_bps"] and not r.get("suspended")
        tracks.setdefault(r["receiver"], Track(r["receiver"])).add(t, ok)
    return tracks


LOG_RE = re.compile(r'time=(\S+) .*msg="Received video height changed".* peer_id=(\S+) stream=(\d+) previous_height=\d+ height=(\d+) requested_height=(\d+)')


def mediasoup_tracks(report, log_lines):
    tracks = {}
    b = report.get("browser") or {}
    end = parse_time(report["ended_at"])
    if b.get("samples") and b.get("period") and b.get("expected_height"):
        start = parse_time(b["period"]["start"])
        tr = Track("browser")
        for s in b["samples"]:
            hs = s["heights"]
            tr.add(start + datetime.timedelta(seconds=s["t"]), bool(hs) and all(h == b["expected_height"] for h in hs))
        tr.first = start  # From the page's load
        tracks["browser"] = tr
    heights, requested = {}, {}
    for line in log_lines:
        m = LOG_RE.search(line)
        if not m:
            continue
        t, peer, stream, h, req = parse_time(m[1]), m[2], int(m[3]), int(m[4]), int(m[5])
        requested[peer] = req
        heights.setdefault(peer, {})[stream] = h
        ok = all(v == req for v in heights[peer].values())
        tracks.setdefault(peer, Track(peer)).add(t, ok)
    for tr in tracks.values():
        tr.last = max(tr.last, end) if tr.last else end
    return tracks, requested


def table(tracks, only=None):
    browser = tracks.get("browser")
    b_reached = browser.reached() if browser else None
    rows = ["| Receiver | Reached its need after | Under it after the browser + 1 s | Held low |", "|---|---|---|---|"]
    held = []
    for name in sorted(tracks, key=lambda n: (n != "browser", n)):
        if only is not None and name != "browser" and name not in only:
            continue
        tr = tracks[name]
        r = tr.reached()
        reached = f"{(r - tr.first).total_seconds():.1f} s" if r else "never"
        if name == "browser" or b_reached is None:
            rows.append(f"| {name} | {reached} | | |")
            continue
        under = tr.under_after(b_reached + datetime.timedelta(seconds=MARGIN), tr.last)
        low = under > 0
        if low:
            held.append(name)
        rows.append(f"| {name} | {reached} | {under:.1f} s | {'yes' if low else 'no'} |")
    if b_reached is None:
        rows.append("\nThe browser never reached its need: no bot is compared with it.")
    else:
        rows.append(f"\nBots held low: {len(held)} ({', '.join(held) if held else 'none'}).")
    return "\n".join(rows), held


def self_test():
    here = os.path.join(os.path.dirname(os.path.abspath(__file__)), "testdata")
    with open(os.path.join(here, "estimates-sample.jsonl")) as f:
        out, held = table(jitsi_tracks(f))
    assert held == ["bbbbbbbb"], (held, out)
    assert "| aaaaaaaa | 1.0 s | 0.0 s | no |" in out, out
    assert "| bbbbbbbb | never | 3.0 s | yes |" in out, out
    with open(os.path.join(here, "heights-sample.json")) as f:
        report = json.load(f)
    with open(os.path.join(here, "heights-sample.log")) as f:
        tracks, requested = mediasoup_tracks(report, f)
    only = {p for p, h in requested.items() if h == 720}
    out, held = table(tracks, only)
    assert held == ["bot-1"], (held, out)
    assert "bot-0" not in out, out  # Asked for 180: not compared
    assert "| browser | 2.0 s | | |" in out, out
    print("self-test ok")


def main(argv):
    if argv[1:] == ["--self-test"]:
        return self_test()
    if len(argv) == 3 and argv[1] == "jitsi":
        with open(argv[2]) as f:
            print(table(jitsi_tracks(f))[0])
        return
    if len(argv) == 4 and argv[1] == "mediasoup":
        with open(argv[2]) as f:
            report = json.load(f)
        with open(argv[3]) as f:
            tracks, requested = mediasoup_tracks(report, f)
        top = max(requested.values(), default=0)
        print(f"Bots compared: those asking for {top}.\n")
        print(table(tracks, {p for p, h in requested.items() if h == top})[0])
        return
    sys.exit(__doc__)


if __name__ == "__main__":
    main(sys.argv)
