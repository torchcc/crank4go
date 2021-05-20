package protocol

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/torchcc/crank4go-core/util"
)

const (
	SupportingHttpVersion                 = "HTTP/1.1"
	CrankerProtocolVersion10              = "1.0"
	CrankerProtocolDebugHeader            = "X-ROUTER_DEBUG_MODE"
	CrankerProtocolDebugHeaderValueEnable = "1"

	RequestBodyPendingMarker = "_1"
	RequestHasNoBodyMarker   = "_2"
	RequestBodyEndedMarker   = "_3"
)

func ValidateCrankerProtocolVersion(version string) bool {
	if version == "" {
		return false
	}
	if version != CrankerProtocolVersion10 {
		return false
	}
	util.LOG.Debugf("i can establish connection with Cranker Protocol %s, currently support %s", CrankerProtocolVersion10, SupportingHttpVersion)
	return true
}

func IsDebugResp(response *CrankerProtocolResponse) bool {
	if response == nil {
		return false
	}
	return checkDebugHeader(response.Headers)
}

func IsDebugReq(request *CrankerProtocolRequest) bool {
	if request == nil {
		return false
	}
	return checkDebugHeader(request.Headers)
}

func checkDebugHeader(headersArr []string) bool {
	if headersArr == nil || len(headersArr) == 0 {
		return false
	}
	pattern := CrankerProtocolDebugHeader + ":"
	for _, headerNameValue := range headersArr {
		if strings.HasPrefix(headerNameValue, pattern) {
			nameValue := strings.Split(headerNameValue, ":")
			if len(nameValue) == 2 && strings.TrimSpace(nameValue[1]) == CrankerProtocolDebugHeaderValueEnable {
				return true
			}
		}
	}
	return false
}

// ################
type HeadersBuilder struct {
	headers string
}

func (builder *HeadersBuilder) AppendHeader(header, value string) {
	builder.headers += header + ":" + value + "\n"
}

func (builder *HeadersBuilder) AppendHeaders(headers []string) {
	for _, header := range headers {
		builder.headers += header + "\n"
	}
}

func (builder *HeadersBuilder) String() string {
	return builder.headers
}

// #################
type CrankerProtocolResponseBuilder struct {
	sourceUrl  string
	httpMethod string
	status     int
	reason     string
	headers    *HeadersBuilder
}

func (b *CrankerProtocolResponseBuilder) WithRespStatus(status int) *CrankerProtocolResponseBuilder {
	b.status = status
	return b
}

func (b *CrankerProtocolResponseBuilder) WithRespReason(reason string) *CrankerProtocolResponseBuilder {
	b.reason = reason
	return b
}

func (b *CrankerProtocolResponseBuilder) WithRespHeaders(headers *HeadersBuilder) *CrankerProtocolResponseBuilder {
	b.headers = headers
	return b
}

func (b *CrankerProtocolResponseBuilder) WithSourceUrl(requestDest string) *CrankerProtocolResponseBuilder {
	b.sourceUrl = requestDest
	return b
}

func (b *CrankerProtocolResponseBuilder) WithHttpMethod(method string) *CrankerProtocolResponseBuilder {
	b.httpMethod = method
	return b
}

func (b *CrankerProtocolResponseBuilder) Build() string {
	return fmt.Sprintf("HTTP/1.1 %d %s \n%s %s\n%s", b.status, b.reason, b.httpMethod, b.sourceUrl, b.headers.String())
}

// ############
type CrankerProtocolRequestBuilder struct {
	reqLine   string
	headers   *HeadersBuilder
	endMarker string
}

func (b *CrankerProtocolRequestBuilder) WithReqLine(line string) *CrankerProtocolRequestBuilder {
	b.reqLine = line
	return b
}

func (b *CrankerProtocolRequestBuilder) WithReqHeaders(headers *HeadersBuilder) *CrankerProtocolRequestBuilder {
	b.headers = headers
	return b
}

func (b *CrankerProtocolRequestBuilder) WithReqBodyPending() *CrankerProtocolRequestBuilder {
	b.endMarker = RequestBodyPendingMarker
	return b
}

func (b *CrankerProtocolRequestBuilder) WithReqHasNoBody() *CrankerProtocolRequestBuilder {
	b.endMarker = RequestHasNoBodyMarker
	return b
}

func (b *CrankerProtocolRequestBuilder) WithReqBodyEnded() *CrankerProtocolRequestBuilder {
	b.endMarker = RequestBodyEndedMarker
	return b
}

func (b *CrankerProtocolRequestBuilder) Build() string {
	if b.reqLine != "" && b.headers != nil {
		return b.reqLine + "\n" + b.headers.String() + "\n" + b.endMarker
	}
	return b.endMarker
}

// ################
// define an interface
type CrankerProtocolMessage interface {
	ToProtocolMessage() string
}

/*
* CRANKER PROTOCOL_ VERSION_1_0
* request msg format:
* <p>
* ==== msg without body =====
* ** GET /modules/uui-allocation/1.0.68/uui-allocation.min.js.map HTTP/1.1\n
* ** [headers]\n
* ** \n
* ** endmarker
* <p>
* <p>
* OR
* <p>
* ==== msg with body part 1 ====
* ** GET /modules/uui-allocation/1.0.68/uui-allocation.min.js.map HTTP/1.1\n
* ** [headers]\n
* ** \n
* ** endmarker
* ==== msg with body part 2 ====
** [BINARY BODY]
* ==== msg with body part 3 ====
* ** endmarker
 */

// define a implementation of the interface
type CrankerProtocolRequest struct {
	HttpMethod  string
	Dest        string
	Headers     []string
	endMarker   string
	requestLine string
}

func NewCrankerProtocolRequest(msg string) *CrankerProtocolRequest {
	req := new(CrankerProtocolRequest)
	if msg == RequestBodyEndedMarker {
		req.endMarker = msg
	} else {
		msgArr := strings.Split(msg, "\n")
		req.requestLine = msgArr[0]
		util.LOG.Debugf("requestLine >>> %s", req.requestLine)
		bits := strings.Split(req.requestLine, " ")
		req.HttpMethod = bits[0]
		req.Dest = bits[1]
		req.Headers = make([]string, len(msgArr)-2)
		copy(req.Headers, msgArr[1:len(msgArr)-1])
		util.LOG.Debugf("headers >>> %#v", req.Headers)
		req.endMarker = msgArr[len(msgArr)-1]
		util.LOG.Debugf("marker >>> %s", req.endMarker)
	}
	return req
}

func (req *CrankerProtocolRequest) RequestBodyPending() bool {
	return req.endMarker == RequestBodyPendingMarker
}

func (req *CrankerProtocolRequest) RequestBodyEnded() bool {
	return req.endMarker == RequestBodyEndedMarker
}

func (req *CrankerProtocolRequest) RequestHasNoBody() bool {
	return req.endMarker == RequestHasNoBodyMarker
}

// return rawMsg
func (req *CrankerProtocolRequest) ToProtocolMessage() string {
	if req.requestLine != "" && req.Headers != nil {
		headersStr := ""
		for _, hl := range req.Headers {
			headersStr += hl + "\n"
		}
		return req.requestLine + "\n" + headersStr + req.endMarker
	} else {
		return req.endMarker
	}
}

type RequestCallback interface {
	callback()
}

func (req *CrankerProtocolRequest) String() string {
	return "CrankerProtocolRequest{" + req.HttpMethod + " " + req.Dest + "}"
}

/**
* CRANKER PROTOCOL_ VERSION_1_0
* <p>
* response msg format:
* <p>
* ==== part 1 ====
* ** HTTP/1.1 200 OK\n
* ** GET /appstore/api/health
* ** [headers]\n
* ** \n
* ==== part 2 (if msg has body) ====
* ** Binary Content
 */

type CrankerProtocolResponse struct {
	Headers    []string
	status     int
	reason     string
	sourceUrl  string
	httpMethod string
}

func NewCrankerProtocolResponse(msg string) *CrankerProtocolResponse {
	resp := new(CrankerProtocolResponse)
	msgArr := strings.Split(msg, "\n")
	bits := strings.Split(msgArr[0], " ")
	resp.status, _ = strconv.Atoi(bits[1])
	if len(bits) >= 3 {
		resp.reason = bits[2]
	}
	originalRequest := msgArr[1]
	requestBits := strings.Split(originalRequest, " ")
	resp.httpMethod = requestBits[0]
	if len(requestBits) >= 2 {
		resp.sourceUrl = requestBits[1]
	}
	resp.Headers = make([]string, 0, len(msgArr)-2)
	copy(resp.Headers, msgArr[2:])
	return resp
}

func (resp *CrankerProtocolResponse) GetSourceUrl() string {
	return resp.sourceUrl
}

func (resp *CrankerProtocolResponse) GetHttpMethod() string {
	return resp.httpMethod
}

func (resp *CrankerProtocolResponse) Status() int {
	return resp.status
}

func (resp *CrankerProtocolResponse) ToProtocolMessage() string {
	builder := new(HeadersBuilder)
	builder.AppendHeaders(resp.Headers)
	return new(CrankerProtocolResponseBuilder).
		WithHttpMethod(resp.httpMethod).
		WithSourceUrl(resp.sourceUrl).
		WithRespReason(resp.reason).
		WithRespStatus(resp.status).
		WithRespHeaders(builder).Build()
}

func (resp *CrankerProtocolResponse) String() string {
	return "CrankerProtocolResponse{" +
		"headers=" + fmt.Sprintf("%s", resp.Headers) +
		", status=" + strconv.Itoa(resp.status) +
		", reason=" + resp.reason +
		", sourceUrl=" + resp.sourceUrl +
		", httpMethod=" + resp.httpMethod +
		"}"
}
