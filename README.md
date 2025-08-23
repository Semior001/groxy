<div align="center">

<img class="logo" src=".github/logo.png" width="334px" height="108px" alt="gRoxy | gRPC mocking server"/>

[![build](https://github.com/Semior001/groxy/actions/workflows/.go.yaml/badge.svg)](https://github.com/Semior001/groxy/actions/workflows/.go.yaml)&nbsp;[![Coverage Status](https://coveralls.io/repos/github/Semior001/groxy/badge.svg?branch=master)](https://coveralls.io/github/Semior001/groxy?branch=master)&nbsp;[![Go Report Card](https://goreportcard.com/badge/github.com/Semior001/groxy)](https://goreportcard.com/report/github.com/Semior001/groxy)&nbsp;[![Go Reference](https://pkg.go.dev/badge/github.com/Semior001/groxy.svg)](https://pkg.go.dev/github.com/Semior001/groxy)&nbsp;[![GitHub release](https://img.shields.io/github/release/Semior001/groxy.svg)](https://github.com/Semior001/groxy/releases)

</div>

gRoxy is a gRPC mocking server that allows you to mock gRPC services and responses easily by specifying the message content alongside the message definition. gRoxy is designed to be used in development and testing environments to help you test your gRPC clients and services without having to rely on the actual gRPC server.

* * *

- [todos](#todos)
- [installation](#installation)
- [usage](#usage)
  - [example](#example)
  - [configuration](#configuration)
  - [gRPC reflection](#grpc-reflection)
  - [groxypb](#groxypb)
    - [multiline-strings](#multiline-strings)
    - [templating](#templating)
    - [nested messages](#nested-messages)
    - [enums](#enums)
    - [repeated fields](#repeated-fields)
    - [maps](#maps)
- [benchmark](#benchmark)
  - [mocker](#mocker)
  - [reverse-proxy](#reverse-proxy) 
- [project status](#status)

## todos
- [ ] currently service supports only mocking unary methods, but we plan to support streaming methods as well

## installation
You can install gRoxy using the following command:

```shell
go install github.com/Semior001/groxy/cmd/groxy@latest
```

Or you can pull the docker image from ghcr.io:

```shell
docker pull ghcr.io/semior001/groxy:latest
```

Or from the docker hub:

```shell
docker pull semior/groxy:latest
```

## usage

```
Usage:
  groxy [OPTIONS]

Application Options:
  -a, --addr=                Address to listen on (default: :8080) [$ADDR]
      --stdin                Read configuration from stdin instead of file [$STDIN]
      --signature            Enable gRoxy signature headers [$SIGNATURE]
      --reflection           Enable gRPC reflection merger [$REFLECTION]
      --json                 Enable JSON logging [$JSON]
      --debug                Enable debug mode [$DEBUG]

file:
      --file.name=           Config file name (default: groxy.yml) [$FILE_NAME]
      --file.check-interval= Check interval for the config file (default: 3s) [$FILE_CHECK_INTERVAL]
      --file.delay=          Delay before applying the changes (default: 500ms) [$FILE_DELAY]

Help Options:
  -h, --help                 Show this help message
```

### example
The simplest configuration for a method "Stub" would look like this:

```yaml
version: 1

rules:
  - match: { uri: "com.github.Semior001.groxy.example.mock.ExampleService/Stub" }
    respond:
      body: |
        message StubResponse {
            option              (groxypb.target) = true; // this option specifies that the message is a response 
            string message = 1 [(groxypb.value)  = "Hello, World!"];
            int32 code     = 2 [(groxypb.value)  = "200"];
        }
```

That's it. You just need to define the response message, mark it as a target response message via the option, and set values via the value option. The response message will be sent to the client when the client calls the "Stub" method. No need for providing protosets, no need for providing the whole set of definitions, just the message you want to send.

More importantly, if your response message contains lots of fields, which are not important for the test, you can just ignore them. gRoxy will leave them empty, and the client will not be able to distinguish between the real server and the mock server. That's ensured by the protobuf's backward compatibility.

Field is backward compatible if it's of the same type and the same number. Names of the fields and messages are not important.


### configuration
gRoxy uses a YAML configuration file to define the rules for the gRPC mocking server. The configuration file consists of the following sections:

| Section     | Description                                                                                                                                                                                          |
|-------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| version     | The version of the configuration file.<br/>The current version is `1`, and any other version will raise an error.                                                                                    |
| not-matched | The not-matched section contains the default response if the request didn't match to any rule. Not-matched section may contain a request body, or a gRPC status. <br/><br/> See respond type section |
| upstreams   | The upstreams section contains the list of the upstreams that serve gRPC reflection services.                                                                                                        |
| rules       | The rules section contains the rules for the gRPC mocking server.                                                                                                                                    |

Upstreams section is a key-value map of upstreams, where key is the name of the upstream to be referenced further in the rules section. Each upstream consists of the following fields:

| Field            | Required | Description                                                                                                                                                 |
|------------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| address          | true     | The address of the upstream gRPC reflection service.                                                                                                        |
| tls              | optional | The TLS configuration for the upstream. The TLS configuration consists of the following fields:                                                             |
| serve-reflection | optional | The flag that indicates whether the upstream's responses should be included in the gRPC reflection responses. No-op if `--reflection` flag is not provided. |

Rules are defined in the rules section. Either `respond` or `forward` must be defined Each rule consists of the following fields:

| Field            | Required | Description                                                                                                                   |
|------------------|----------|-------------------------------------------------------------------------------------------------------------------------------|
| match            | true     | The match section contains the matchers for the request.                                                                      |
| match.uri        | true     | The URI matcher for the request. The URI matcher is a regular expression that matches the URI of the request.                 |
| match.header     | optional | a map of headers that should be present in the request.                                                                       |
| match.body       | optional | The body matcher for the request. This must be a protobuf snippet that defines the request message with values to be matched. |
| respond          | optional | The respond section contains the response for the request.                                                                    |
| forward          | optional | The forward section contains the upstream to which request should be forwarded to.                                            |

The `Respond` section contains the response for the request. The respond section may contain the following fields:

| Field       | Required                   | Description                                                                                                         |
|-------------|----------------------------|---------------------------------------------------------------------------------------------------------------------|
| body        | optional                   | The body of the response. This must be a protobuf snippet that defines the response message with values to be sent. |
| metadata    | optional                   | The metadata to be sent as a response.                                                                              |
| status      | optional                   | The gRPC status to be sent as a response.                                                                           |
| status.code | true, if status is present | The gRPC status code to be sent as a response.                                                                      |
| status.msg  | true, if status is present | The gRPC status message to be sent as a response.                                                                   |

The `Forward` section contains the upstream to which the request should be forwarded. The forward section may contain the following fields:

| Field    | Required | Description                                                                                                                                       |
|----------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| upstream | true     | The name of the upstream to which the request should be forwarded. Supports templating: use `env` function to get the environment variable value. |
| header   | optional | The headers to be sent with the request.                                                                                                          |

The configuration file is being watched for changes, and the server will reload the configuration file if it changes.

You can also take a look at [examples](_example) for more information.

### gRPC reflection
gRoxy supports gRPC reflection services. If you want to merge the responses from the upstream gRPC reflection services, you need to provide the `--reflection` flag and set the `serve-reflection` flag to `true` on the upstreams that should be included in the reflection responses.

The reflection responses are merged in the following way:
1. Services are merged and unified by the package + service name.
2. File descriptors are merged into a single array among the upstreams.
3. `AllExtensionNumbersOfType` responds with the first non-error response from the upstreams.

### groxypb
gRoxy uses the `groxypb` annotations to define values in protobuf message snippets. It compiles protobuf in a runtime, checking the target via the `groxypb.target` option and interpreting values via the `groxypb.value` option.

Example of the snippet:
```protobuf
message SomeMessage {
    option              (groxypb.target) = true; 
    string message = 1 [(groxypb.value) = "Hello, World!"];
    int32 code = 2     [(groxypb.value) = "200"];
}
```

#### multiline strings
protobuf itself doesn't support multiline strings, so gRoxy introduces it's own syntax for them in order to allow to specify complex values in `groxypb.value` option. Multiline strings should be enclosed in backticks:
```protobuf
message Dependency {
  string field1 = 1;
  int32  field2 = 2;
  bool   field3 = 3;
}

message SomeMessage {
    option              (groxypb.target) = true; 
    string message = 1 [(groxypb.value) = `Hello, 
    World!`];
    Dependency dependency = 2 [(groxypb.value) = `{
        "field1": "value1",
        "field2": 2,
        "field3": true
    }`];
}
```

#### templating
gRoxy supports Go templating in the `groxypb.value` field annotations, allowing dynamic response generation. Templates can access request data and use a variety of built-in functions.

##### template functions
- **Sprig functions**: All [Sprig template functions](https://masterminds.github.io/sprig/).
- **Request data access**: Use `.fieldName` to access data from the incoming request

##### examples

```protobuf
message EnvResponse {
    option (groxypb.target) = true;
    string env_value = 1 [(groxypb.value) = "{{env \"API_VERSION\"}}"];
}

message DynamicResponse {
    option (groxypb.target) = true;
    string result = 1 [(groxypb.value) = "Result: {{upper (printf \"num-%d\" (mul .factor 2))}}"];
}
```

##### request matching with expressions
You can use the `groxypb.matcher` option to conditionally match requests based on field values:

```protobuf
message TestRequest {
    option (groxypb.target) = true;
    string type = 1 [(groxypb.value) = "premium"];           // exact match
    int32 score = 2 [(groxypb.matcher) = "score > 100"];     // conditional match
}
```

The matcher uses the [expr](https://github.com/expr-lang/expr) language for evaluations.

#### nested messages
In case of nested messages, there are two options how to set values:
1. Set the value in the nested message:
```protobuf
message SomeMessage {
    option                    (groxypb.target) = true; 
    string parent_value = 1   [(groxypb.value) = "parent"];
    NestedMessage nested = 2;
}

message NestedMessage {
    string nested_value = 1 [(groxypb.value) = "nested"];
}
```

2. Set the JSON value in the annotation to the parent's field:
```protobuf
message SomeMessage {
    option                    (groxypb.target) = true; 
    string parent_value = 1  [(groxypb.value) = "parent"];
    NestedMessage nested = 2 [(groxypb.value) = '{"nested_value": "nested"}'];
}
```

Depending on your use case, you may find different approaches useful. For instance, if you can have multiple objects of the same type, you may specify values in the options of the nested message definition itself rather than in the parent message. On the other hand, defining an exact value in the parent message is useful if you want to get different values for the nested message in different fields.

#### enums

For enums, the value should be set as a string name of the enum value:
```protobuf
enum SomeEnum { 
    EMPTY = 0; 
    NEEDED_VALUE = 2; 
}

message StubResponse {
    option (groxypb.target) = true;
    SomeEnum some_enum = 9 [(groxypb.value) = 'NEEDED_VALUE'];
}
```

#### repeated fields
For repeated fields, the value should be set as a JSON array of the values:
```protobuf
message StubResponse {
    option (groxypb.target) = true;
    repeated string repeated_field = 1 [(groxypb.value) = '["value1", "value2"]'];
}
```

#### maps
For maps, the value should be set as a JSON object of the key-value pairs:
```protobuf
message StubResponse {
    option (groxypb.target) = true;
    map<string, string> map_field = 1 [(groxypb.value) = '{"key1": "value1", "key2": "value2"}'];
}
```

## benchmark

all benchmarks were performed on a MacBook Pro 2021 with M1 Pro chip with 16GB of RAM.

### mocker

single rule on mock

```shell
$ ghz --insecure --call 'grpc_echo/v1/EchoService/Echo' -d '{"ping": "Hello, world!"}' -c 1 --total 10000 localhost:8080

Summary:
  Count:	10000
  Total:	3.42 s
  Slowest:	8.03 ms
  Fastest:	0.12 ms
  Average:	0.24 ms
  Requests/sec:	2923.93

Response time histogram:
  0.119 [1]    |
  0.909 [9965] |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
  1.700 [30]   |
  2.491 [0]    |
  3.282 [2]    |
  4.072 [1]    |
  4.863 [0]    |
  5.654 [0]    |
  6.444 [0]    |
  7.235 [0]    |
  8.026 [1]    |

Latency distribution:
  10 % in 0.17 ms 
  25 % in 0.18 ms 
  50 % in 0.21 ms 
  75 % in 0.26 ms 
  90 % in 0.35 ms 
  95 % in 0.42 ms 
  99 % in 0.66 ms 

Status code distribution:
  [OK]   10000 responses   
```

### reverse-proxy

performed with the use of [grpc-echo](https://github.com/Semior001/grpc-echo), single rule on upstream

```shell
$ ghz --insecure --call 'grpc_echo/v1/EchoService/Echo' -d '{"ping": "Hello, world!"}' -c 1 --total 10000 localhost:8080

Summary:
  Count:	10000
  Total:	10.16 s
  Slowest:	43.56 ms
  Fastest:	0.46 ms
  Average:	0.90 ms
  Requests/sec:	984.55

Response time histogram:
  0.456  [1]    |
  4.767  [9980] |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
  9.078  [16]   |
  13.389 [0]    |
  17.700 [0]    |
  22.010 [1]    |
  26.321 [1]    |
  30.632 [0]    |
  34.943 [0]    |
  39.254 [0]    |
  43.564 [1]    |

Latency distribution:
  10 % in 0.61 ms 
  25 % in 0.67 ms 
  50 % in 0.78 ms 
  75 % in 0.97 ms 
  90 % in 1.25 ms 
  95 % in 1.53 ms 
  99 % in 2.57 ms 

Status code distribution:
  [OK]   10000 responses   
```

## status
The project is currently in active development, and breaking changes may occur until the release of version 1.0. However, we strive to minimize disruptions and will only introduce breaking changes when there is a compelling reason to do so.
