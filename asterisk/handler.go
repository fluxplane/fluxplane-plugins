package asterisk

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	plugin := pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(amiPingSpec(), service.AMIPing),
		pluginbinding.RegisterOperation(channelListSpec(), service.ChannelList),
		pluginbinding.RegisterOperation(channelHangupSpec(), service.Hangup),
		pluginbinding.RegisterOperation(peerListSpec(), service.PeerList),
		pluginbinding.RegisterOperation(queueStatusSpec(), service.QueueStatus),
		pluginbinding.RegisterOperation(deviceStateListSpec(), service.DeviceStateList),
		pluginbinding.RegisterOperation(commandSpec(), service.Command),
		pluginbinding.RegisterOperation(originateSpec(), service.Originate),
	)
	plugin.Command(protocol.CommandEndpointsDiscover, service.DiscoverEndpointsCommand)
	return plugin
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
