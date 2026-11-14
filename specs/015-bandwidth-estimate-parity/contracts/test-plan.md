# Contract: `browser` and `server_settings` in the test plan (015)

Two optional sections of `test_plan`, beside `media` and
`server_monitoring`. Both are platform-independent: no adapter reads them.
A test plan without them is valid and runs as before (FR-018).

## `browser`

```json
"browser": {
  "url": "https://localhost:8443/load015#config.prejoinConfig.enabled=false",
  "expected_height": 720,
  "window": "1280x720",
  "insecure_tls": true
}
```

| Field | Type | Default | Meaning |
|---|---|---|---|
| `url` | string | required | address the browser opens. The engine performs no step in the page: it must join the room by itself |
| `expected_height` | integer, 1–2160 | none | height the browser's share of time is measured against; without it the share is left out with `not_stated` |
| `window` | string `<w>x<h>`, `320x240`–`3840x2160` | `1280x720` | window size; pages pick the heights they ask for from it |
| `insecure_tls` | boolean | `false` | accept a certificate the browser does not trust |
| `executable` | string | found in `PATH` | path of the Chrome or Chromium program |

Refused at creation, naming the field: `url` missing, not `http`/`https`, or
without a host; `expected_height` or `window` out of range; unknown fields.

Not refused: a missing browser or a wrong address. They are reasons in the
run's report, and the run goes on (FR-019).

One browser per run: the section is an object, not a list.

In the report and in the API's answers, the values of the `url`'s query and
fragment parameters `jwt`, `token` and `access_token` are replaced by
`[redacted]` (`scenario.Redacted`, as secret `platform_config` values).

## `server_settings`

```json
"server_settings": {
  "source": "operator",
  "values": {"JVB_CC_TRUST_BWE": "false"}
}
```

| Field | Type | Default | Meaning |
|---|---|---|---|
| `source` | `operator` or `deployment` | `operator` | who stated the settings |
| `values` | object, string → string, at most 16 | empty | settings of the server under test that differ from its defaults and change results, named as the server names them |

Refused at creation: another `source`; more than 16 values; an empty name;
`source: operator` with no value (it states nothing).

`source: deployment` with no value is valid: the deployment changed nothing.

The engine does not check the values against the server. They are stated,
and the report gives them as stated (US4 scenario 2).

## Where the values come from

| Way of running | `browser` | `server_settings` |
|---|---|---|
| API (`POST /api/v1/scenarios`) | the section | the section |
| Live tests | `<PLATFORM>_BROWSER_URL`, `<PLATFORM>_BROWSER_HEIGHT`, `<PLATFORM>_BROWSER_WINDOW`, `<PLATFORM>_BROWSER_INSECURE_TLS=1` | `<PLATFORM>_SERVER_SETTINGS` when set, as `source: deployment` |
| `jitsi-connect` | `--browser-url`, `--browser-height`, `--browser-window`, `--browser-insecure-tls` | `--server-settings name=value,...` (`operator`), or `$JITSI_SERVER_SETTINGS` (`deployment`) |

## Settings file of `sfu-remote` (adds to 013's
[settings-file.md](../../013-remote-sfu-deployment/contracts/settings-file.md))

`deploy` writes one more variable per server:

```sh
JITSI_SERVER_SETTINGS='JVB_CC_TRUST_BWE=false'
LIVEKIT_SERVER_SETTINGS=''
```

`name=value` pairs separated by commas; present and empty when the remote
overlay changes no setting that changes results. A settings file written
before this feature has no such variable: the run then reports the server's
settings as not known.
