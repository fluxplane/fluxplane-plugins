package kubernetes

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	plugin := pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(clusterListSpec(), service.ClusterList),
		pluginbinding.RegisterOperation(clusterTestSpec(), service.ClusterTest),
		pluginbinding.RegisterOperation(endpointDiscoverSpec(), service.EndpointDiscover),
		pluginbinding.RegisterOperation(namespaceListSpec(), service.NamespaceList),
		pluginbinding.RegisterOperation(serviceListSpec(), service.ServiceList),
		pluginbinding.RegisterOperation(serviceShowSpec(), service.ServiceShow),
		pluginbinding.RegisterOperation(podListSpec(), service.PodList),
		pluginbinding.RegisterOperation(podShowSpec(), service.PodShow),
		pluginbinding.RegisterOperation(podLogsSpec(), service.PodLogs),
		pluginbinding.RegisterOperation(portForwardStartSpec(), service.PortForwardStart),
		pluginbinding.RegisterOperation(portForwardStopSpec(), service.PortForwardStop),
		pluginbinding.RegisterOperation(portForwardListSpec(), service.PortForwardList),
		pluginbinding.RegisterOperation(deploymentListSpec(), service.DeploymentList),
		pluginbinding.RegisterOperation(deploymentShowSpec(), service.DeploymentShow),
		pluginbinding.RegisterOperation(deploymentHistorySpec(), service.DeploymentHistory),
		pluginbinding.RegisterOperation(deploymentScaleSpec(), service.DeploymentScale),
		pluginbinding.RegisterOperation(deploymentRestartSpec(), service.DeploymentRestart),
		pluginbinding.RegisterOperation(ingressListSpec(), service.IngressList),
		pluginbinding.RegisterOperation(containerListSpec(), service.ContainerList),
		pluginbinding.RegisterOperation(containerShowSpec(), service.ContainerShow),
		pluginbinding.RegisterOperation(eventListSpec(), service.EventList),
		pluginbinding.RegisterOperation(nodeListSpec(), service.NodeList),
		pluginbinding.RegisterOperation(podExecSpec(), service.PodExec),
		pluginbinding.RegisterDatasourceSearch(inventoryDatasourceSpec(), service.InventorySearch),
	)
	plugin.Command(protocol.CommandEndpointsDiscover, service.DiscoverEndpointsCommand)
	return plugin
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
