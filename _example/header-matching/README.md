# Header Matching

Demonstrates matching requests by both HTTP headers and request body fields.

## What it does

The rule matches requests to `ExampleService/Stub` that have:
- Header `test: true`
- Body field `message` equal to `"needed value"`

When both conditions match, it responds with `{"message": "needed value received", "code": 200}`.

## Config walkthrough

```yaml
match:
  uri: "...ExampleService/Stub"
  header: { test: true }       # match on header
  body: |                       # match on body field
    message StubRequest {
        option (groxypb.target) = true;
        string message = 1 [(groxypb.value) = "needed value"];
    }
```

## Run

Start the server:
```sh
task server CONFIG=_example/header-matching/config.yaml
```

Send a request:
```sh
grpcurl -plaintext -proto '_example/service.proto' \
  -H 'test: true' \
  -d '{"message":"needed value"}' \
  localhost:8080 com.github.Semior001.groxy.example.mock.ExampleService/Stub
```

Expected response:
```json
{"message": "needed value received", "code": 200}
```
