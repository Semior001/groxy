<div align="center">

<img class="logo" src=".github/logo.png" width="334px" height="108px" alt="gRoxy | gRPC mocking server"/>

[![build](https://github.com/Semior001/groxy/actions/workflows/.go.yaml/badge.svg)](https://github.com/Semior001/groxy/actions/workflows/.go.yaml)&nbsp;[![Coverage Status](https://coveralls.io/repos/github/Semior001/groxy/badge.svg?branch=master)](https://coveralls.io/github/Semior001/groxy?branch=master)&nbsp;[![Go Report Card](https://goreportcard.com/badge/github.com/Semior001/groxy)](https://goreportcard.com/report/github.com/Semior001/groxy)&nbsp;[![Go Reference](https://pkg.go.dev/badge/github.com/Semior001/groxy.svg)](https://pkg.go.dev/github.com/Semior001/groxy)&nbsp;[![GitHub release](https://img.shields.io/github/release/Semior001/groxy.svg)](https://github.com/Semior001/groxy/releases)

</div>

gRoxy is a gRPC mocking server that allows you to mock gRPC services and responses easily by specifying the message content alongside the message definition. gRoxy is designed to be used in development and testing environments to help you test your gRPC clients and services without having to rely on the actual gRPC server.

## example
```yaml
version: 1

# by default, groxy will respond with the status code INTERNAL and the message "didn't match the request to any rule".
not-matched:
  status: { code: "NOT_FOUND", message: "not found" }

rules:
    # The next rule will respond with a predefined message.
  - match: { uri: "com.github.Semior001.groxy.example.mock.ExampleService/Stub" }
    respond:
      body: |
        message StubResponse {
            // this option specifies that the message is a response
            option              (groxypb.target) = true; 
            string message = 1 [(groxypb.value)  = "Hello, World!"];
            int32 code     = 2 [(groxypb.value)  = "200"];
        }
```

