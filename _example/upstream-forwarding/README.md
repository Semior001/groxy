# Upstream Forwarding

Demonstrates forwarding a gRPC request to an upstream service with injected headers.

## What it does

Matches requests to `grpc_echo.v1.EchoService/Echo` and forwards them to the configured upstream (`grpc-echo.semior.dev:443`), adding an `X-Request-Id` header.

## Config walkthrough

```yaml
upstreams:
  echo:
    address: "grpc-echo.semior.dev:443"
    tls: true
    serve-reflection: true

rules:
  - match: { uri: "grpc_echo.v1.EchoService/Echo" }
    forward:
      upstream: echo
      header:
        X-Request-Id: "123"
```

## Run

Start the server:
```sh
task server CONFIG=_example/upstream-forwarding/config.yaml
```

Send a request (uses reflection from the upstream):
```sh
grpcurl -plaintext -use-reflection \
  localhost:8080 grpc_echo.v1.EchoService/Echo
```

Expected: response from the upstream echo service.
