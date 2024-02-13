package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

type (
	JsonRPC interface {
		//Register a service
		Register(srv any) error

		//Register a service and specify name
		RegisterWithName(srv any, name string) error

		// The `ServeHTTP` function is responsible for handling incoming JSON-RPC requests. It takes in an
		// `http.ResponseWriter` and an `http.Request` as parameters.
		ServeHTTP(w http.ResponseWriter, r *http.Request)
	}

	//Used to service to method name and request object in batch request's go routine
	batchServiceRequestType struct {
		methodName string
		req        request
		service    *service
	}

	//Type for error channel in service.call routine. It maps err to error code and request ID
	callerError struct {
		err   error
		code  RpcErrorCode
		reqId *string
	}

	//Type for response channel in service.call routine. It maps response data to request ID
	callerSuccess struct {
		data  any
		reqId *string
	}

	//JSON rpc request object type
	request struct {
		Id      *string `json:"id,omitempty"` //Id of request. Can be nil if it is a notification
		Method  string  `json:"method"`       //Method name. Should be  service.method. eg. Arith.Add
		Params  []any   `json:"params"`       //Argument of method
		Jsonrpc string  `json:"jsonrpc"`      //RPC version. Should be 2.0
	}

	//JSON RPC error response object type
	errorResponse struct {
		Code    RpcErrorCode `json:"code"`    //A Number that indicates the error type that occurred.
		Data    any          `json:"data"`    //A Primitive or Structured value that contains additional information about the error. This may be omitted.
		Message string       `json:"message"` //A String providing a short description of the error.
	}

	//json RPC response type
	response struct {
		Jsonrpc string         `json:"jsonrpc"`          //RPC version. Should be 2.0
		Id      *string        `json:"id,omitempty"`     //Id of request. Can be nil if it is a notification
		Result  *any           `json:"result,omitempty"` //Results,Should be empty if error is not
		Error   *errorResponse `json:"error,omitempty"`  //Results,Should be empty if Result is not
	}

	//A service is a group of related methods
	service struct {
		methods map[string]reflect.Value
		name    string
	}

	//RPC implementation
	jsonRpcImpl struct {
		services map[string]*service
	}
)

func NewJsonRpc() JsonRPC {
	return &jsonRpcImpl{
		services: make(map[string]*service),
	}
}

func (rpc *jsonRpcImpl) register(srv any, name *string) error {
	if reflect.ValueOf(srv).NumMethod() == 0 {
		return errors.New("No method registered for this service")
	}

	service := new(service)
	service.methods = make(map[string]reflect.Value, 0)

	if name == nil {
		service.name = reflect.ValueOf(srv).Type().Name()
	} else {
		service.name = *name
	}

	for m := 0; m < reflect.ValueOf(srv).NumMethod(); m++ {
		methodVal := reflect.ValueOf(srv).Method(m)
		method := reflect.ValueOf(srv).Type().Method(m)

		if isValidMethod(method) {
			methodName := method.Name
			service.methods[methodName] = methodVal
		}

	}

	rpc.services[service.name] = service

	if len(rpc.services) == 0 {
		return errors.New("No method registered for this service")
	}

	return nil
}

func (rpc *jsonRpcImpl) Register(srv any) error {
	return rpc.register(srv, nil)
}

func (rpc *jsonRpcImpl) RegisterWithName(srv any, name string) error {
	return rpc.register(srv, &name)
}

// Call this in a go routine
func (s service) call(ctx context.Context, methodName string, args []any, id *string, respChan chan callerSuccess, errChan chan callerError) {
	method, ok := s.methods[methodName]
	if !ok {
		err := errors.New(fmt.Sprintf("Method %s does not exist on service %s", methodName, s.name))
		errChan <- callerError{
			err:   err,
			code:  METHOD_NOT_FOUND,
			reqId: id,
		}

		return
	}

	params := []reflect.Value{reflect.ValueOf(ctx)}
	for _, arg := range args {
		params = append(params, reflect.ValueOf(arg))
	}

	//Handle panics from reflect
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("Recovered from panic:", err)
			errChan <- callerError{
				err:   errors.New(fmt.Sprintf("Internal error: Panic %s", err)),
				code:  INTERNAL_ERROR,
				reqId: id,
			}
		}
	}()

	//Call method
	resp := method.Call(params)
	if resp[1].Interface() != nil {

		errCode := resp[2].Interface()
		var code RpcErrorCode

		if errCode == nil {
			code = INTERNAL_ERROR
		} else {
			code = *errCode.(*RpcErrorCode)
		}

		errorResponse := resp[1].Interface().(error)

		errChan <- callerError{
			err:   errorResponse,
			code:  code,
			reqId: id,
		}
		return
	}

	respChan <- callerSuccess{
		data:  resp[0].Interface(),
		reqId: id,
	}

	return
}

// Decode json request to be either single or batch request type
func readRequest(r *http.Request) (*request, []request, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, err
	}

	singleRequest := &request{}
	if err := json.Unmarshal(body, singleRequest); err == nil {
		//single request
		return singleRequest, nil, nil
	}

	batchRequest := &[]request{}
	if err := json.Unmarshal(body, batchRequest); err == nil {
		//batch request
		return nil, *batchRequest, nil
	}

	return nil, nil, errors.New("Unable to decode request")
}

func writeResponse(w http.ResponseWriter, res response, id *string) {
	// Request is notification
	if id == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// I cannot handle another error here
	r, _ := json.Marshal(&res)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(r)
}

func writeBatchResponse(w http.ResponseWriter, responses []response) {

	//Filter responses for all requests that are not notifications
	validResponses := make([]response, 0)
	for _, resp := range responses {
		if resp.Id != nil {
			validResponses = append(validResponses, resp)
		}
	}

	r, _ := json.Marshal(&validResponses)

	w.WriteHeader(http.StatusOK)
	w.Write(r)
}

func writeSuccessResponse(w http.ResponseWriter, data any, id *string) {
	writeResponse(w, makeSuccessResponse(&data, id), id)
}

func writeErrorResponse(w http.ResponseWriter, err error, errCode RpcErrorCode, id *string, data any) {
	writeResponse(w, makeErrorResponse(err, errCode, &data, id), id)
}

// The function `sanitizeMethodPath` splits a method name into a service name and a method name, and
// returns them along with an error if the method name is invalid.
func sanitizeMethodPath(method string) (serviceName *string, methodName *string, err error) {
	if !strings.Contains(method, ".") {
		err = errors.New("Invalid method name")
		return
	}

	n := strings.Split(method, ".")

	serviceName = &n[0]
	methodName = &n[1]
	err = nil

	return
}

func (s *jsonRpcImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r)
}

func makeErrorResponse(err error, errCode RpcErrorCode, data *any, id *string) response {

	return response{
		Jsonrpc: RPC_VERSION,
		Id:      id,
		Result:  nil,
		Error: &errorResponse{
			Code:    errCode,
			Message: err.Error(),
			Data:    data,
		},
	}
}

func makeSuccessResponse(data *any, id *string) response {

	return response{
		Jsonrpc: RPC_VERSION,
		Id:      id,
		Result:  data,
		Error:   nil,
	}
}

func (s *jsonRpcImpl) handleBatchRequest(ctx context.Context, w http.ResponseWriter, requests []request) {
	responses := make([]response, 0)

	validServices := make([]batchServiceRequestType, 0)

	for _, req := range requests {
		if req.Jsonrpc != RPC_VERSION {
			err := errors.New("Invalid RPC version. jsonrpc must be 2.0")
			responses = append(responses, makeErrorResponse(err, INVALID_REQUEST, nil, req.Id))

			continue
		}

		serviceName, methodName, err := sanitizeMethodPath(req.Method)

		if err != nil {
			responses = append(responses, makeErrorResponse(err, PARSE_ERROR, nil, req.Id))
			continue
		}

		service, ok := s.services[*serviceName]

		if !ok {
			err = errors.New(fmt.Sprintf("Service %s is not registered", *serviceName))
			responses = append(responses, makeErrorResponse(err, METHOD_NOT_FOUND, nil, req.Id))
			continue
		}
		validServices = append(validServices, batchServiceRequestType{req: req, service: service, methodName: *methodName})
	}

	var mu sync.Mutex
	respChan := make(chan callerSuccess)
	errChan := make(chan callerError)

	for _, s := range validServices {
		go s.service.call(ctx, s.methodName, s.req.Params, s.req.Id, respChan, errChan)
	}

	for range validServices {
		select {
		case e := <-errChan:
			mu.Lock()
			responses = append(responses, makeErrorResponse(e.err, e.code, nil, e.reqId))
			mu.Unlock()

		case r := <-respChan:
			mu.Lock()
			responses = append(responses, makeSuccessResponse(&r.data, r.reqId))
			mu.Unlock()

		case <-ctx.Done():
			err := errors.New("Request was not able to complete")
			mu.Unlock()
			responses = append(responses, makeErrorResponse(err, INTERNAL_ERROR, nil, nil))
			mu.Unlock()
		}
	}

	close(respChan)
	close(errChan)

	writeBatchResponse(w, responses)

}

func (s *jsonRpcImpl) handleSingleRequest(ctx context.Context, w http.ResponseWriter, req request) {

	if req.Jsonrpc != RPC_VERSION {
		err := errors.New("Invalid RPC version. jsonrpc must be 2.0")
		writeErrorResponse(w, err, INVALID_REQUEST, req.Id, nil)
		return
	}

	serviceName, methodName, err := sanitizeMethodPath(req.Method)

	if err != nil {
		writeErrorResponse(w, err, PARSE_ERROR, req.Id, nil)
		return
	}

	service, ok := s.services[*serviceName]

	if !ok {
		err = errors.New(fmt.Sprintf("Service %s is not registered", *serviceName))
		writeErrorResponse(w, err, METHOD_NOT_FOUND, req.Id, nil)

		return
	}

	respChan := make(chan callerSuccess)
	errChan := make(chan callerError)

	//Call method in a go routine
	go service.call(ctx, *methodName, req.Params, req.Id, respChan, errChan)

	select {
	case err := <-errChan:
		writeErrorResponse(w, err.err, err.code, err.reqId, nil)

	case d := <-respChan:
		writeSuccessResponse(w, d.data, d.reqId)

	case <-ctx.Done():
		err := errors.New("Request canceled")
		writeErrorResponse(w, err, INTERNAL_ERROR, req.Id, nil)
	}

	close(respChan)
	close(errChan)
	return
}

func (s *jsonRpcImpl) handle(w http.ResponseWriter, r *http.Request) {
	singleRequest, batchRequest, err := readRequest(r)

	if err != nil {
		writeErrorResponse(w, err, PARSE_ERROR, nil, nil)
		return
	}

	//Handle request types
	if singleRequest != nil {
		s.handleSingleRequest(r.Context(), w, *singleRequest)
		return
	}

	s.handleBatchRequest(r.Context(), w, batchRequest)

}

func isValidMethod(methodType reflect.Method) bool {
	if !methodType.IsExported() {
		return false
	}

	if methodType.Type.NumIn() == 0 {
		return false
	}
	if methodType.Type.NumOut() != 3 {
		return false
	}

	return true
}
