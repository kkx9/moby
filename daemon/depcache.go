package daemon

import (
	containertypes "github.com/docker/docker/api/types/container"
	_"github.com/docker/docker/depcache"
	"github.com/docker/docker/layer"
	_"github.com/sirupsen/logrus"
)

func (d *Daemon) AddLayer(layerDigest string, depLayers []string, config *containertypes.Config, cacheID string) {
	d.depCache.AddLayer(layerDigest, depLayers, config, cacheID)
}

func (d *Daemon) CheckLayer(config *containertypes.Config, depLayers []string, imageLayers *[]layer.DiffID, cacheIDList *[]string) string {
	return d.depCache.CheckLayer(config, depLayers, imageLayers, cacheIDList)
}