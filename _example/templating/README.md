# Response Templating

Demonstrates dynamic response generation using Go templates.

## What it does

Two rules are shown:

1. **Random UUID** — responds with a randomly generated UUID when `message` is `"random"`.
2. **Expression matcher with arithmetic** — when `message` is `"matcher"` and `multiplier > 0`, waits 2 seconds and responds with `multiplier * 2`.

## Config walkthrough

```yaml
# Rule 1: UUID generation
respond:
  body: |
    ...string message = 1 [(groxypb.value) = "{{uuidv4}}"];

# Rule 2: arithmetic template + wait
match:
  body: |
    ...int32 multiplier = 2 [(groxypb.matcher) = "multiplier > 0"];
respond:
  wait: 2s
  body: |
    ...string message = 1 [(groxypb.value) = "{{mul .multiplier 2}}"];
```

## Run

Start the server:
```sh
task server CONFIG=_example/templating/config.yaml
```

Random UUID:
```sh
grpcurl -plaintext -proto '_example/service.proto' \
  -d '{"message":"random"}' \
  localhost:8080 com.github.Semior001.groxy.example.mock.ExampleService/Stub
```

Expression matcher (waits 2s):
```sh
grpcurl -plaintext -proto '_example/service.proto' \
  -d '{"message":"matcher","multiplier":5}' \
  localhost:8080 com.github.Semior001.groxy.example.mock.ExampleService/Stub
```

Expected response: `{"message": "10"}`
