package docker

import (
	"context"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type Client interface {
	Close() error
	Info(context.Context) (DockerInfo, error)
	ListContainers(context.Context, ContainerListInput) ([]Container, error)
	InspectContainer(context.Context, string) (Container, error)
	ContainerLogs(context.Context, ContainerLogsInput) (ContainerLogsResult, error)
	ContainerStats(context.Context, ContainerStatsInput) (ContainerStatsResult, error)
	ContainerTop(context.Context, ContainerTopInput) (ContainerTopResult, error)
	ContainerExec(context.Context, ContainerExecInput) (ContainerExecResult, error)
	ContainerCopyFrom(context.Context, ContainerCopyFromInput) (ContainerCopyResult, error)
	ContainerCopyTo(context.Context, ContainerCopyToInput) (ContainerCopyResult, error)
	ContainerCreate(context.Context, ContainerCreateInput) (ContainerCreateResult, error)
	ContainerRun(context.Context, ContainerCreateInput) (ContainerCreateResult, error)
	ContainerStart(context.Context, ContainerStartInput) (ContainerActionResult, error)
	ContainerStop(context.Context, ContainerStopInput) (ContainerActionResult, error)
	ContainerRestart(context.Context, ContainerRestartInput) (ContainerActionResult, error)
	ContainerRemove(context.Context, ContainerRemoveInput) (ContainerActionResult, error)
	ContainerInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	ContainerPrune(context.Context, PruneInput) (PruneResult, error)
	ListImages(context.Context, ImageListInput) ([]Image, error)
	InspectImage(context.Context, string) (Image, error)
	ImagePull(context.Context, ImagePullInput) (ImagePullResult, error)
	ImageTag(context.Context, ImageTagInput) (ResourceActionResult, error)
	ImagePush(context.Context, ImagePushInput) (ImagePushResult, error)
	ImageBuild(context.Context, ImageBuildInput) (ImageBuildResult, error)
	ImageRemove(context.Context, ImageRemoveInput) (ImageRemoveResult, error)
	ImageInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	ImagePrune(context.Context, ImagePruneInput) (ImagePruneResult, error)
	ListNetworks(context.Context, NetworkListInput) ([]Network, error)
	InspectNetwork(context.Context, string) (Network, error)
	NetworkCreate(context.Context, NetworkCreateInput) (ResourceActionResult, error)
	NetworkRemove(context.Context, NetworkRemoveInput) (ResourceActionResult, error)
	NetworkInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	NetworkPrune(context.Context, PruneInput) (PruneResult, error)
	SystemDF(context.Context, SystemDFInput) (SystemDFResult, error)
	SystemPrune(context.Context, SystemPruneInput) (SystemPruneResult, error)
	Events(context.Context, EventsInput) (EventsResult, error)
	ListVolumes(context.Context, VolumeListInput) ([]Volume, error)
	InspectVolume(context.Context, string) (Volume, error)
	VolumeCreate(context.Context, VolumeCreateInput) (Volume, error)
	VolumeRemove(context.Context, VolumeRemoveInput) (ResourceActionResult, error)
	VolumeInspectRaw(context.Context, RawInspectInput) (RawInspectResult, error)
	VolumePrune(context.Context, PruneInput) (PruneResult, error)
	BuildCachePrune(context.Context, BuildCachePruneInput) (PruneResult, error)
	ContextList(context.Context, ContextListInput) ([]DockerContext, error)
	ContextShow(context.Context, ContextShowInput) (DockerContext, error)
}

// ClientFactory builds a Docker client for an invocation. The live
// implementation (NewLiveClient) dials the daemon through the host conn
// capability; tests substitute a fake.
type ClientFactory func(pluginbinding.Context) (Client, error)
