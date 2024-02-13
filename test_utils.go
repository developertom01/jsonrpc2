package jsonrpc2

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
)

func makeRpcSingleTestRequest(rpc JsonRPC, req request) (*response, error) {
	recorder := httptest.NewRecorder()

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("POST", "/", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	rpc.ServeHTTP(recorder, r)

	body, err := io.ReadAll(recorder.Body)
	if err != nil {
		return nil, err
	}

	var statusCode = recorder.Result().StatusCode
	if statusCode == int(http.StatusOK) || statusCode == int(http.StatusNoContent) {
		res := &response{}
		if err := json.Unmarshal(body, res); err != nil {
			return nil, err
		}

		return res, nil
	}

	return nil, errors.New("Unsuccessfully RPC request")

}

func makeRpcBatchTestRequest(rpc JsonRPC, reqs []request) ([]response, error) {
	recorder := httptest.NewRecorder()

	reqBody, err := json.Marshal(reqs)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("POST", "/", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	rpc.ServeHTTP(recorder, r)

	body, err := io.ReadAll(recorder.Body)
	if err != nil {
		return nil, err
	}

	var statusCode = recorder.Result().StatusCode
	if statusCode == int(http.StatusOK) || statusCode == int(http.StatusNoContent) {
		res := &[]response{}
		if err := json.Unmarshal(body, res); err != nil {
			return nil, err
		}

		return *res, nil
	}

	return nil, errors.New("Unsuccessfully RPC request")

}
