package image // import "github.com/docker/docker/image"

import (
	"runtime"

	"github.com/docker/docker/layer"
	"github.com/sirupsen/logrus"
)

// TypeLayers is used for RootFS.Type for filesystems organized into layers.
const TypeLayers = "layers"

// typeLayersWithBase is an older format used by Windows up to v1.12. We
// explicitly handle this as an error case to ensure that a daemon which still
// has an older image like this on disk can still start, even though the
// image itself is not usable. See https://github.com/docker/docker/pull/25806.
const typeLayersWithBase = "layers+base"

// RootFS describes images root filesystem
// This is currently a placeholder that only supports layers. In the future
// this can be made into an interface that supports different implementations.
type RootFS struct {
	Type    	string         	`json:"type"`
	DiffIDs 	[]layer.DiffID 	`json:"diff_ids,omitempty"`
	DepChainID	layer.ChainID	`json:"chain_id,omitempty"`
}

// NewRootFS returns empty RootFS struct
func NewRootFS() *RootFS {
	return &RootFS{Type: TypeLayers}
}

// Append appends a new diffID to rootfs
func (r *RootFS) Append(id layer.DiffID) {
	r.DiffIDs = append(r.DiffIDs, id)
	if string(r.DepChainID) != "" {
		logrus.Debugf("append update depchainID: %s, %s", string(r.DepChainID), string(id))
		r.DepChainID = layer.CreateChainID([]layer.DiffID{layer.DiffID(r.DepChainID), id})
		logrus.Debug(r.DepChainID)
	}
}

// Clone returns a copy of the RootFS
func (r *RootFS) Clone() *RootFS {
	newRoot := NewRootFS()
	newRoot.Type = r.Type
	newRoot.DiffIDs = make([]layer.DiffID, len(r.DiffIDs))
	newRoot.DepChainID = r.DepChainID
	copy(newRoot.DiffIDs, r.DiffIDs)
	return newRoot
}

// ChainID returns the ChainID for the top layer in RootFS.
func (r *RootFS) ChainID() layer.ChainID {
	if runtime.GOOS == "windows" && r.Type == typeLayersWithBase {
		logrus.Warnf("Layer type is unsupported on this platform. DiffIDs: '%v'", r.DiffIDs)
		return ""
	}
	if string(r.DepChainID) == "" {
		return layer.CreateChainID(r.DiffIDs)
	}
	logrus.Debug("rootfs getid: ",r.DepChainID)
	return r.DepChainID
}
