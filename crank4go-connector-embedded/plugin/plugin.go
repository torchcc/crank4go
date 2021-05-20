package plugin

import (
	. "github.com/torchcc/crank4go-core/cranker-protocol"
	"github.com/torchcc/crank4go-core/util"
)

type ConnectorPluginStatCarrier interface {
	// the stat that connector plugin carries from connector socket context to 2 plugins method in a connectorPlugin
	GetStat() interface{}

	// close the stat when a plugin method finished its job
	Close() error
}

type ConnectorPlugin interface {
	// run the plugin method before request is sent to local service connector after connector accepted the cranker router
	// eg: sometime when connector side need to send some event to notify / log the status of request processing
	HandleBeforeRequestSent(req *CrankerProtocolRequest, carrier ConnectorPluginStatCarrier) (ConnectorPluginStatCarrier, error)

	// run the plugin method after local service connector response received
	// eg: sometime when connector side need to send some event to notify / log the status of request processing
	HandleAfterResponseReceived(resp *CrankerProtocolResponse, carrier ConnectorPluginStatCarrier) (ConnectorPluginStatCarrier, error)
}

type ConnectorPluginStatCarriers struct {
	statCarrierMap map[ConnectorPlugin]ConnectorPluginStatCarrier
}

func (cs *ConnectorPluginStatCarriers) Close() {
	var err error
	for _, carrier := range cs.statCarrierMap {
		util.LOG.Debugf("closing carrier %s", carrier)
		if err = carrier.Close(); err != nil {
			util.LOG.Warningf("failed to close carrier, err: %s", err.Error())
		}
	}
}

func (cs *ConnectorPluginStatCarriers) Put(plugin ConnectorPlugin, carrier ConnectorPluginStatCarrier) {
	if plugin != nil && carrier != nil {
		cs.statCarrierMap[plugin] = carrier
	}
}

func (cs *ConnectorPluginStatCarriers) Get(plugin ConnectorPlugin) ConnectorPluginStatCarrier {
	if c, ok := cs.statCarrierMap[plugin]; ok {
		return c
	}
	return nil
}

func (cs *ConnectorPluginStatCarriers) CleanCarrier(plugin ConnectorPlugin) {
	if carrier, ok := cs.statCarrierMap[plugin]; ok {
		_ = carrier.Close()
		delete(cs.statCarrierMap, plugin)
	}
}

func NewConnectorPluginStatCarriers() *ConnectorPluginStatCarriers {
	return &ConnectorPluginStatCarriers{statCarrierMap: make(map[ConnectorPlugin]ConnectorPluginStatCarrier)}
}
