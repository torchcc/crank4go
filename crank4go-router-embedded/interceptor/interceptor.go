package interceptor

import (
	"net/http"

	ptc "github.com/torchcc/crank4go-core/cranker-protocol"
	"github.com/torchcc/crank4go-core/util"
)

type InterceptorStatCarrier interface {
	Close() error
}

type ProxyInterceptor interface {
	/*
		Apply intercepting logic on original HTTPRequest and cranker protocol request created fromm current request context.
		e.g. when we need to modified http header or protocol request
	*/
	ApplyOnReq(req *http.Request, ptcReq *ptc.CrankerProtocolRequest) (InterceptorStatCarrier, error)
}

type InterceptorStatCarriers struct {
	statCarrierMap map[ProxyInterceptor]InterceptorStatCarrier
}

func NewInterceptorStatCarriers() *InterceptorStatCarriers {
	return &InterceptorStatCarriers{statCarrierMap: map[ProxyInterceptor]InterceptorStatCarrier{}}
}

func (c *InterceptorStatCarriers) Close() error {
	for _, carrier := range c.statCarrierMap {
		util.LOG.Infof("closing carrier %s", carrier)
		if err := carrier.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *InterceptorStatCarriers) Add(interceptor ProxyInterceptor, carrier InterceptorStatCarrier) {
	if interceptor != nil && carrier != nil {
		c.statCarrierMap[interceptor] = carrier
	}
}

func (c *InterceptorStatCarriers) CleanCarrier(p ProxyInterceptor) {
	if carrier, ok := c.statCarrierMap[p]; ok {
		_ = carrier.Close()
		delete(c.statCarrierMap, p)
	}
}
