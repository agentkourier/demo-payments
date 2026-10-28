# payments

The payments service authorizes card payments for orders. Checkout calls it once per order.

Owned by the payments team. Part of the `commerce` system.

## API

### `POST /v1/authorize`

Request:

```json
{"order_id": "ord_1001", "amount_cents": 4250, "currency": "USD"}
```

`order_id` is required, `amount_cents` must be positive, and `currency` is an upper-case
ISO 4217 code. Invalid requests get `400` with `{"error": "..."}`.

Response (`200`):

```json
{
  "authorization_id": "auth_7QZK2M4XJ5RVT3WA",
  "order_id": "ord_1001",
  "status": "approved",
  "amount": "42.50",
  "currency": "USD"
}
```

`amount` is a decimal string with two places.

### Operational endpoints

| Endpoint       | Purpose                                                   |
|----------------|-----------------------------------------------------------|
| `GET /healthz` | Liveness: `200` while the process is up.                  |
| `GET /readyz`  | Readiness: `200` once serving, `503` after SIGTERM.        |
| `GET /metrics` | Prometheus metrics.                                       |

## Configuration

| Variable                      | Default | Description                                                    |
|-------------------------------|---------|----------------------------------------------------------------|
| `ADDR`                        | `:8080` | Listen address.                                                |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset   | OTLP gRPC endpoint for traces. Tracing is off when it is unset. |

The standard `OTEL_*` exporter and resource variables also apply. Spans carry
`service.name=payments` and `service.version` set to the build version.

## Telemetry

- Metrics: `http_requests_total{handler,code}` and `http_request_duration_seconds{handler}`,
  plus the Go runtime and process collectors.
- Traces: one server span per request (probes and `/metrics` excluded), W3C trace context.
- Logs: JSON on stdout, one line per request; lines logged within a traced request include
  `trace_id` and `span_id`.

## Build and run

```sh
make test                      # go vet and go test -race
make build                     # bin/payments, version from git describe
make run                       # go run on :8080
make image VERSION=v2.13.0     # docker image demo-payments:v2.13.0
```

The version is set at build time with `-ldflags "-X main.version=..."`; the image
carries it in the `org.opencontainers.image.version` label and the commit in
`org.opencontainers.image.revision`.
