# JSON RPC VERSION 2.0 implementation

This package implements the `JSON-RPC` version 2.0 spec.

[![Go Reference](https://pkg.go.dev/badge/github.com/developertom01/jsonrpc2.svg)](https://pkg.go.dev/github.com/developertom01/jsonrpc2)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Coverage Status](https://coveralls.io/repos/github/developertom01/jsonrpc2/badge.svg?branch=main)](https://coveralls.io/github/developertom01/jsonrpc2?branch=main)

## Procedure signature

It is important to not how to structure your procedure.

To understand how using `jsonrpc2` works you need to understand what is service and method

- Service is a group of related methods. A service should be of a struct type.
eg.

```go

type AuthService struct{}

authService := new(AuthService)
```

- A Method is an individual exported function on a service struct.
For a receiver function to be considered a valid receiver function it should obey the following rules.
  - The receiver should be exported. In Golang exported function names begin with an uppercase alphabet.
  - The receiver function should accept context as the first argument and every function should take in at least the context param.
  - The receiver function should return 3 values. `Return value` if there is no error, `Error` if any and `Error code` if there is an error.

- Example of a valid Service

```go

type UserService struct {
  db Database
}

//Valid method
func (u UserService) FullName(ctx context.Context)(string, error, RpcErrorCode){
  //Impl...
  return fmt.Sprintf("%s %s",firstName,lastName), nil,nil
}

//Invalid method
func (u UserService) SetNames(fullName){
  //Impl...
}

//Invalid method
func (u UserService) setNames(fullName){
  //Impl...
}

//Valid method
func (u UserService) UpdateFirstName(ctx context.Context, userId int, firstName string) (User,error,RpcErrorCode){
  return nil, errors.New("Some error"), INTERNAL_ERROR
}


```

## Test

### Setup

```go
package main

import (
 "context"
 "fmt"
 "net/http"

 jsonrpc2 "github.com/developertom01/jsonrpc2"
)

type Arithmetic struct{}

func (Arithmetic) Add(ctx context.Context, a, b float64) (float64, error, *jsonrpc2.RpcErrorCode) {
 return a + b, nil, nil
}

func (Arithmetic) Sub(ctx context.Context, a, b float64) (float64, error, *jsonrpc2.RpcErrorCode) {
 return a - b, nil, nil
}

func main() {

 rpc := jsonrpc2.NewJsonRpc()

 arith := Arithmetic{}
 rpc.Register(arith)

 if err := http.ListenAndServe(":8000", rpc); err != nil {
  panic(fmt.Sprintf("Server failed to start: %s", err.Error()))
 }

}

```

- Single request

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":"2","method":"Arithmetic.Add","params":[1,2]}' \
  http://localhost:8000
```

- Batch Request

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '[
        {
            "jsonrpc":"2.0",
            "id":"1",
            "method":"Arithmetic.Add",
            "params":[1,2]
        },
        {
            "jsonrpc":"2.0",
            "id":"2",
            "method":"Arithmetic.Sub",
            "params":[1,1]
        }
      ]' \
  http://localhost:8000
```
