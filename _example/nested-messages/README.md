# Nested Messages

Demonstrates setting nested message fields in a mock response using JSON in `groxypb.value`.

## What it does

Matches any request to `ExampleService/Stub` and responds with a `StubResponse` containing a nested `Dependency` message populated via JSON.

## Config walkthrough

```yaml
respond:
  body: |
    message Dependency {
        string some_dependant_value = 6;
        bool   some_dependant_bool  = 7;
        string some_rich_text       = 8;
    }

    message StubResponse {
        option (groxypb.target) = true;
        Dependency dependency = 3 [(groxypb.value) = `{
            "some_dependant_value": "some value",
            "some_dependant_bool":  true,
            "some_rich_text":       "some text"
        }`];
    }
```

## Run

Start the server:
```sh
task server CONFIG=_example/nested-messages/config.yaml
```

Send a request:
```sh
grpcurl -plaintext -proto '_example/service.proto' \
  localhost:8080 com.github.Semior001.groxy.example.mock.ExampleService/Stub
```

Expected response:
```json
{"dependency": {"value": "some value", "flag": true, "richText": "some text"}}
```
