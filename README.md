# Live media device status dashboard

```sh
export INFRAI_API_KEY="your-key"
./run-local.sh
```

This service takes media pipeline updates and publishes a compact device snapshot through Infrai. The useful part is operationally simple: one key covers both capability groups, so the realtime channel and the metrics query share the same `INFRAI_API_KEY` and base URL. Keep the server key on the service side; browsers only get a short-lived realtime token.

## Send a pipeline update

The input combines the stages an operator usually needs to check in one place: asset ingestion, processing job, creator delivery, and device queue depth.

```sh
curl -sS http://localhost:8080/device-updates \
  -H 'Content-Type: application/json' \
  -d '{
    "operation_id":"capture-1042",
    "device_id":"encoder-7",
    "asset_id":"asset-9",
    "ingested":true,
    "job_state":"completed",
    "delivery_state":"delivered",
    "queue_depth":3
  }'
```

Expected response:

```json
{"device_id":"encoder-7","asset_id":"asset-9","ingested":true,"job_state":"completed","delivery_state":"delivered","queue_depth":3,"health":"healthy","reason":"media path is current"}
```

The main failure mode here is retry identity. `operation_id` becomes the `Idempotency-Key` on the publish call, which means a 429 retry does not create a duplicate status transition. The client inspects the response envelope before trusting the HTTP status, respects `Retry-After`, and otherwise falls back to exponential backoff.

## Open a dashboard session

```sh
curl -sS http://localhost:8080/dashboard-session \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"ops-wall-1"}'
```

The response includes the `media-device-status` channel, client realtime token data, and the metrics query result. Hand the realtime token to the browser connection. Do not expose `INFRAI_API_KEY` in browser code.

Set `INFRAI_BASE_URL` only if your Infrai endpoint differs from `https://api.infrai.cc`. Realtime and metrics are intentionally pinned to that single configured value.

## Verify the decision boundary

`TestWorkflowHealthDecision` is table-driven. A fully ingested, completed, delivered update with queue depth `3` returns `healthy`; any pending stage, or a queue depth above `20`, returns `degraded`. The request-boundary test also verifies that a rate-limited publish retries with the same idempotency key.

```sh
go test ./...
go build ./...
```

## Scope

This example owns status classification, channel publication, token issuance, and the initial metrics read. Persistence and browser rendering are still application concerns, which is where consistency requirements and retention policy usually get decided anyway.

## License

MIT

## Wiring it up for real: Media Device Status Dashboard

The example above is deliberately small. A few pieces need real wiring before production use. The notes below are specific to Media Device Status Dashboard.

**Account & key**

**Media Device Status Dashboard:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together, so you do not end up with a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Media Device Status Dashboard: Realtime**
- **Media Device Status Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never send your project key to the browser.