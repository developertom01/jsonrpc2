package jsonrpc2

// 32000 to 32099	Server error	Reserved for implementation-defined server-errors.
type RpcErrorCode int

const (
	PARSE_ERROR      RpcErrorCode = 32700
	INVALID_REQUEST  RpcErrorCode = 32600
	METHOD_NOT_FOUND RpcErrorCode = 32601
	INVALID_PARAMS   RpcErrorCode = 32602
	INTERNAL_ERROR   RpcErrorCode = 32603
)
