package jsonrpc2

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestNewJsonRpc(t *testing.T) {
	rpc := NewJsonRpc()
	assert.Implements(t, (*JsonRPC)(nil), rpc)
}

type arith struct{}

func (arith) Add(ctx context.Context, a, b float64) (int, error, *RpcErrorCode) {
	var errorCode = INTERNAL_ERROR
	return int(a + b), nil, &errorCode
}

func (arith) ErrorMethod(ctx context.Context) (*int, error, *RpcErrorCode) {
	var errorCode = INTERNAL_ERROR
	return nil, errors.New("Some error here"), &errorCode
}

type testType struct{}

func (testType) FuncCheck1() {}

func (testType) FuncCheck2(context.Context) {}

// Insufficient Output
func (testType) FuncCheck3(context.Context) (string, error) {
	return "", nil
}

// Invalid type
func (testType) FuncCheck4(context.Context) (string, error, int) {
	return "", nil, 1
}

// Invalid type
func (testType) FuncCheck5(context.Context) (string, error, *RpcErrorCode) {
	var errCode = INTERNAL_ERROR
	return "", nil, &errCode
}

func TestIsValidMethod(t *testing.T) {
	methodType1 := reflect.ValueOf(testType{}).Type().Method(0)
	isValid := isValidMethod(methodType1)
	assert.False(t, isValid)

	methodType2 := reflect.ValueOf(testType{}).Type().Method(1)
	isValid2 := isValidMethod(methodType2)
	assert.False(t, isValid2)

	methodType3 := reflect.ValueOf(testType{}).Type().Method(2)
	isValid3 := isValidMethod(methodType3)
	assert.False(t, isValid3)

	methodType5 := reflect.ValueOf(testType{}).Type().Method(4)
	isValid5 := isValidMethod(methodType5)
	assert.True(t, isValid5)

}

func TestRegister(t *testing.T) {
	rpc := &jsonRpcImpl{
		services: make(map[string]*service),
	}

	rpc.register(arith{}, nil)
	service, ok := rpc.services["arith"]

	assert.True(t, ok)

	_, ok = service.methods["Add"]

	assert.True(t, ok)
}

func TestRegisterWithName(t *testing.T) {
	rpc := &jsonRpcImpl{
		services: make(map[string]*service),
	}
	var name = "Arith"

	rpc.register(arith{}, &name)
	service, ok := rpc.services[name]

	assert.True(t, ok)

	_, ok = service.methods["Add"]

	assert.True(t, ok)
}

func TestServiceCall(t *testing.T) {
	rpc := &jsonRpcImpl{
		services: make(map[string]*service),
	}

	var (
		id             = "1"
		args           = []any{3, 2}
		expectedOutput = 5

		serviceName = "Arith"
	)

	rpc.register(arith{}, &serviceName)
	service, ok := rpc.services[serviceName]

	assert.True(t, ok)

	respChan := make(chan callerSuccess)
	errChan := make(chan callerError)

	defer func() {
		close(respChan)
		close(errChan)
	}()

	ctx := context.Background()

	go service.call(ctx, "Add", args, &id, respChan, errChan)

	select {
	case r := <-respChan:
		assert.Equal(t, id, *r.reqId)
		assert.Equal(t, expectedOutput, r.data)
	case e := <-errChan:
		assert.Equal(t, id, *e.reqId)
	}
}

type JsonRpc2TestSuite struct {
	suite.Suite
	rpc JsonRPC
}

func (suite *JsonRpc2TestSuite) SetupTest() {
	rpc := NewJsonRpc()
	rpc.RegisterWithName(arith{}, "Arith")

	suite.rpc = rpc
}
func (suit *JsonRpc2TestSuite) TestHandleSingle() {
	var (
		id             = "1"
		expectedOutput = float64(4)
	)

	req := request{
		Id:      &id,
		Method:  "Arith.Add",
		Params:  []any{1, 3},
		Jsonrpc: RPC_VERSION,
	}

	res, err := makeRpcSingleTestRequest(suit.rpc, req)

	if err != nil {
		suit.T().Fatal(err)
	}

	suit.Equal(*res.Id, id)
	suit.Equal(res.Jsonrpc, RPC_VERSION)
	suit.Equal(*res.Result, expectedOutput)

}

func (suit *JsonRpc2TestSuite) TestHandleSingleNoMethod() {
	var (
		id                   = "1"
		expectedErrorMessage = "Method Sub does not exist on service Arith"
	)

	req := request{
		Id:      &id,
		Method:  "Arith.Sub",
		Params:  []any{1, 3},
		Jsonrpc: RPC_VERSION,
	}

	res, err := makeRpcSingleTestRequest(suit.rpc, req)

	if err != nil {
		suit.T().Fatal(err)
	}

	suit.Equal(*res.Id, id)
	suit.Equal(res.Jsonrpc, RPC_VERSION)
	suit.Nil(res.Result)
	suit.Equal(res.Error.Code, METHOD_NOT_FOUND)
	suit.Equal(res.Error.Message, expectedErrorMessage)
}

func (suit *JsonRpc2TestSuite) TestHandleSingleWrongVersion() {
	var (
		id                   = "1"
		WrongJsonRpcVersion  = "1.1"
		expectedErrorMessage = "Invalid RPC version. jsonrpc must be 2.0"
	)

	req := request{
		Id:      &id,
		Method:  "Arith.Add",
		Params:  []any{1, 3},
		Jsonrpc: WrongJsonRpcVersion,
	}

	res, err := makeRpcSingleTestRequest(suit.rpc, req)

	if err != nil {
		suit.T().Fatal(err)
	}

	suit.Equal(*res.Id, id)
	suit.Equal(res.Jsonrpc, RPC_VERSION)
	suit.Nil(res.Result)
	suit.Equal(res.Error.Code, INVALID_REQUEST)
	suit.Equal(res.Error.Message, expectedErrorMessage)
}

func (suit *JsonRpc2TestSuite) TestHandleSingleWrongWrongMethodNameFormat() {
	var (
		id                   = "1"
		expectedErrorMessage = "Invalid method name"
	)

	req := request{
		Id:      &id,
		Method:  "ArithAdd",
		Params:  []any{1, 3},
		Jsonrpc: RPC_VERSION,
	}

	res, err := makeRpcSingleTestRequest(suit.rpc, req)

	if err != nil {
		suit.T().Fatal(err)
	}

	suit.Equal(*res.Id, id)
	suit.Equal(res.Jsonrpc, RPC_VERSION)
	suit.Nil(res.Result)
	suit.Equal(res.Error.Code, PARSE_ERROR)
	suit.Equal(res.Error.Message, expectedErrorMessage)
}

func (suit *JsonRpc2TestSuite) TestHandleSingleErrorHandling() {
	var (
		id = "1"
	)

	req := request{
		Id:      &id,
		Method:  "Arith.ErrorMethod",
		Params:  []any{},
		Jsonrpc: RPC_VERSION,
	}

	res, err := makeRpcSingleTestRequest(suit.rpc, req)

	if err != nil {
		suit.T().Fatal(err)
	}

	suit.Equal(*res.Id, id)
	suit.Equal(res.Jsonrpc, RPC_VERSION)
	suit.Nil(res.Result)
	suit.Equal(res.Error.Code, INTERNAL_ERROR)
}

func (suit *JsonRpc2TestSuite) TestHandleBatch() {
	var (
		ids = []string{"1", "2"}
	)

	req := []request{{
		Id:      &ids[0],
		Method:  "Arith.Add",
		Params:  []any{1, 3},
		Jsonrpc: RPC_VERSION,
	}, {
		Id:      &ids[1],
		Method:  "Arith.Add",
		Params:  []any{1, 4},
		Jsonrpc: RPC_VERSION,
	}}

	responses, err := makeRpcBatchTestRequest(suit.rpc, req)

	if err != nil {
		suit.T().Fatal(err)
	}

	for _, res := range responses {
		suit.Equal(res.Jsonrpc, RPC_VERSION)

	}
	suit.Equal(len(req), len(responses))

}
func TestJsonRpc2(t *testing.T) {

	suite.Run(t, new(JsonRpc2TestSuite))
}
