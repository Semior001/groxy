# URI Rewrite

Demonstrates rewriting the gRPC method URI using regex capture groups before forwarding to an upstream.

## What it does

Matches requests to `ExampleService/Some<Name>Method` (e.g. `SomeEchoMethod`) and rewrites the URI to `EchoService/<Name>` (e.g. `EchoService/Echo`) before forwarding to the upstream.

## Config walkthrough

```yaml
rules:
  - match:
      uri: "...ExampleService/Some([a-zA-Z]+)Method"  # captures "Echo"
    forward:
      rewrite: "grpc_echo.v1.EchoService/$1"           # becomes "EchoService/Echo"
      upstream: echo
```

## Run

Start the server:
```sh
task server CONFIG=_example/uri-rewrite/config.yaml
```

Send a request:
```sh
grpcurl -plaintext -proto '_example/service.proto' \
  localhost:8080 com.github.Semior001.groxy.example.mock.ExampleService/SomeEchoMethod
```

Expected: response from the upstream echo service (the URI was rewritten to `EchoService/Echo`).
