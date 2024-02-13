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
