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
  - [groxypb](#groxypb)
    - [nested messages](#nested-messages)
    - [enums](#enums)
    - [repeated fields](#repeated-fields)
    - [maps](#maps)
- [project status](#status)

## todos
- [ ] currently service supports only mocking unary methods, but we plan to support streaming methods as well
- [ ] serving the gRPC reflection service
- [ ] support for the gRPC health check service
- [ ] passthrough mode for proxying and modifying real gRPC services

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
| rules       | The rules section contains the rules for the gRPC mocking server.                                                                                                                                    |

Rules are defined in the rules section. Each rule consists of the following fields:

| Field            | Required | Description                                                                                                                   |
|------------------|----------|-------------------------------------------------------------------------------------------------------------------------------|
| match            | true     | The match section contains the matchers for the request.                                                                      |
| match.uri        | true     | The URI matcher for the request. The URI matcher is a regular expression that matches the URI of the request.                 |
| match.header     | optional | a map of headers that should be present in the request.                                                                       |
| match.body       | optional | The body matcher for the request. This must be a protobuf snippet that defines the request message with values to be matched. |
| respond          | true     | The respond section contains the response for the request.                                                                    |

The `Respond` section contains the response for the request. The respond section may contain the following fields:

| Field         | Required                   | Description                                                                                                                     |
|---------------|----------------------------|---------------------------------------------------------------------------------------------------------------------------------|
| stream        | optional                   | The stream of responses to be sent as a response.                                                                               |
| stream.def    | true, if stream is present | The stream definition of the response message.                                                                                  |
| stream.values | optional                   | Values of the stream message to be sent to the client. Values are just a set of YAML-specified fields, i.e. `[]map[string]any`. |
| body          | optional                   | The body of the response. This must be a protobuf snippet that defines the response message with values to be sent.             |
| metadata      | optional                   | The metadata to be sent as a response.                                                                                          |
| status        | optional                   | The gRPC status to be sent as a response.                                                                                       |
| status.code   | true, if status is present | The gRPC status code to be sent as a response.                                                                                  |
| status.msg    | true, if status is present | The gRPC status message to be sent as a response.                                                                               |

The configuration file is being watched for changes, and the server will reload the configuration file if it changes.

You can also take a look at [examples](_example) for more information.

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
protobuf itself doesn't support multiline strings, so gRoxy introduces it's own syntax for them in order to allow to specify complex values in `groxypb.value` option. Multiline strings should be enclosed in triple backticks:
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

## status
The project is currently in active development, and breaking changes may occur until the release of version 1.0. However, we strive to minimize disruptions and will only introduce breaking changes when there is a compelling reason to do so.
