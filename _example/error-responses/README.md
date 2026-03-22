# Error Responses

Demonstrates returning gRPC error statuses with custom headers and trailers.

## What it does

Matches any request to `ExampleService/Error` and responds with:
- Status: `INVALID_ARGUMENT` with message `"invalid request"`
- Response header: `X-Request-Id: 123`
- Trailer: `Powered-By: groxy`

## Config walkthrough

```yaml
respond:
  status: { code: "INVALID_ARGUMENT", message: "invalid request" }
  metadata:
    header:  { X-Request-Id: "123" }
    trailer: { Powered-By: "groxy" }
```

## Run

Start the server:
```sh
task server CONFIG=_example/error-responses/config.yaml
```

Send a request (use `-v` to see headers/trailers):
```sh
grpcurl -v -plaintext -proto '_example/service.proto' \
  localhost:8080 com.github.Semior001.groxy.example.mock.ExampleService/Error
```

Expected: `INVALID_ARGUMENT` error with the metadata shown above.
