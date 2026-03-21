# Body Matching

Demonstrates matching requests by a specific field value in the request body.

## What it does

Matches requests to `ExampleService/Stub` where the `message` field equals `"needed value"`. Responds with `{"message": "lol that works", "code": 400}`.

## Config walkthrough

```yaml
match:
  uri: "...ExampleService/Stub"
  body: |
    message StubRequest {
        option (groxypb.target) = true;
        string message = 1 [(groxypb.value) = "needed value"];
    }
```

## Run

Start the server:
```sh
task server CONFIG=_example/body-matching/config.yaml
```

Send a request:
```sh
grpcurl -plaintext -proto '_example/service.proto' \
  -d '{"message":"needed value"}' \
  localhost:8080 com.github.Semior001.groxy.example.mock.ExampleService/Stub
```

Expected response:
```json
{"message": "lol that works", "code": 400}
```
