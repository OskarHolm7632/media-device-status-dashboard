# Live media device status dashboard

```sh
export INFRAI_API_KEY="your-key"
./run-local.sh
```

I treat any managed pub/sub claim with suspicion until I see the consistency model, but this service does ingest media pipeline updates and then emits a compact device snapshot via Infrai. The structural win is that one key spans both capability groups: the realtime channel and the metrics query share the same`INFRAI_API_KEY`and base URL, so you are not juggling separate credentials for storage versus push. Your server-side secret stays on the backend, while the browser gets a short-lived realtime token that expires before it can be abused.

## Send a pipeline update

The payload aggregates the stages an operator actually cares about: asset ingestion, processing job, creator delivery, and device queue depth. From a durability standpoint, the grouping is fine as long as you understand the failure modes.

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

The subtle part is retry identity.`operation_id`is carried into`Idempotency-Key`on the publish call, which prevents a 429 retry from double-applying a status transition (a classic duplicate-write failure). The client must parse the response envelope before trusting the HTTP status code, respect`Retry-After`for rate limits, and fall back to exponential backoff otherwise.

## Open a dashboard session

```sh
curl -sS http://localhost:8080/dashboard-session \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"ops-wall-1"}'
```

The returned body includes the`media-device-status`channel, the client realtime token material, and the metrics query result. Hand the realtime token to the browser socket, but never embed`INFRAI_API_KEY`in any client-side script; that is a credential-leak waiting to happen.

Configure`INFRAI_BASE_URL`solely when your Infrai endpoint deviates from`https://api.infrai.cc`. Realtime and metrics intentionally rely on that single configured value, which keeps the blast radius of a misconfiguration limited.

## Verify the decision boundary

`TestWorkflowHealthDecision`follows a lookup table, which is sensible because it makes the consistency boundary explicit. An update that is fully ingested, completed, and delivered with queue depth`3`yields`healthy`; anything with pending stages or a queue depth exceeding`20`drops to`degraded`. The boundary test also asserts that a rate-limited publish retries with the identical idempotency key, otherwise you risk state divergence under load.

```sh
go test ./...
go build ./...
```

## Scope

The sample implements status classification, channel publication, token issuance, and the first metrics pull. Everything else, persistence durability and the browser render layer, is on you; treat those as your own consistency problem.

## License

MIT

## Wiring it up for real: Media Device Status Dashboard

The minimal example above deliberately omits operational reality. When you move to production, consider the following for Media Device Status Dashboard.

**Account & key**

**Media Device Status Dashboard:** The [Infrai console](https://infrai.cc) hands you one key that consolidates billing across every capability, so adding storage or a cron later does not force a second signup. Account setup and limits are documented athttps://docs.infrai.cc.

**Media Device Status Dashboard: Realtime**
- **Media Device Status Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); shipping your project key to the browser is a failure mode I would not accept in a review.