package plugin

import ptc "github.com/torchcc/crank4go-core/cranker-protocol"

type RouterSocketPlugin interface {
	/*
		handle protocol response on router side.
		protocol response will be received after Cranker connector socket client(the service provider side)
		finish processing requests delegated by cranker router.

		business scenarios example: when we want to extract some information  from the response and use it in downstream
	*/
	HandleAfterRespReceived(resp *ptc.CrankerProtocolResponse) error
}
