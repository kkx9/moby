package depbuilder // import "github.com/docker/docker/builder/depbuilder"

// internals for handling commands. Covers many areas and a lot of
// non-contiguous functionality. Please read the comments.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (b *DepBuilder) getArchiver() *archive.Archiver {
	return chrootarchive.NewArchiver(b.idMapping)
}

func (b *DepBuilder) commit(ctx context.Context, dispatchState *dispatchState, comment string) error {
	if b.disableCommit {
		return nil
	}
	if !dispatchState.hasFromImage() {
		return errors.New("Please provide a source image with `from` prior to commit")
	}

	runConfigWithCommentCmd := copyRunConfig(dispatchState.runConfig, withCmdComment(comment, dispatchState.operatingSystem))
	id, err := b.probeAndCreate(ctx, dispatchState, runConfigWithCommentCmd)
	if err != nil || id == "" {
		return err
	}

	return b.commitContainer(dispatchState, id, runConfigWithCommentCmd)
}

func (b *DepBuilder) commitContainer(dispatchState *dispatchState, id string, containerConfig *container.Config) error {

	if b.disableCommit {
		return nil
	}

	commitCfg := backend.CommitConfig{
		Author: dispatchState.maintainer,
		// TODO: this copy should be done by Commit()
		Config:          copyRunConfig(dispatchState.runConfig),
		ContainerConfig: containerConfig,
		ContainerID:     id,
	}

	// logrus.Debug("commit config: ", commitCfg)

	imageID, err := b.docker.CommitBuildStep(commitCfg)
	dispatchState.imageID = string(imageID)

	flag := b.checkBuildDepdency()
	if len(b.newDepDigest) == 0 {
		b.newDepDigest = append(b.newDepDigest, b.baseImageDigest)
	}

	b.docker.AddLayer(string(imageID), b.newDepDigest, containerConfig, b.traceManager.lastCacheID, flag)

	// logrus.Debug("Add cache")
	// logrus.Debug(string(imageID))
	// logrus.Debug(b.newDepDigest)
	// logrus.Debug(containerConfig)

	return err
}

func (b *DepBuilder) exportImage(state *dispatchState, layer builder.RWLayer, parent builder.Image, runConfig *container.Config) error {
	newLayer, err := layer.Commit()
	if err != nil {
		return err
	}

	cacheID, err := newLayer.CacheID()
	if err != nil {
		return err
	}

	logrus.Debugf("get layer id: %s", cacheID)

	parentImage, ok := parent.(*image.Image)
	if !ok {
		return errors.Errorf("unexpected image type")
	}

	platform := &specs.Platform{
		OS:           parentImage.OS,
		Architecture: parentImage.Architecture,
		Variant:      parentImage.Variant,
	}

	// add an image mount without an image so the layer is properly unmounted
	// if there is an error before we can add the full mount with image
	b.imageSources.Add(newImageMount(nil, newLayer), platform)

	newImage := image.NewChildImage(parentImage, image.ChildConfig{
		Author:          state.maintainer,
		ContainerConfig: runConfig,
		DiffID:          newLayer.DiffID(),
		Config:          copyRunConfig(state.runConfig),
	}, parentImage.OS)

	// TODO: it seems strange to marshal this here instead of just passing in the
	// image struct
	config, err := newImage.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "failed to encode image config")
	}

	exportedImage, err := b.docker.CreateImage(config, state.imageID)
	if err != nil {
		return errors.Wrapf(err, "failed to export image")
	}

	state.imageID = exportedImage.ImageID()
	flag := b.checkBuildDepdency()
	// logrus.Debug("export image")
	b.docker.AddLayer(string(state.imageID), []string{b.baseImageDigest}, runConfig, b.traceManager.lastCacheID, flag)

	b.imageSources.Add(newImageMount(exportedImage, newLayer), platform)
	return nil
}

func (b *DepBuilder) performCopy(ctx context.Context, req dispatchRequest, inst copyInstruction) error {
	state := req.state
	srcHash := getSourceHashFromInfos(inst.infos)

	var chownComment string
	if inst.chownStr != "" {
		chownComment = fmt.Sprintf("--chown=%s", inst.chownStr)
	}
	commentStr := fmt.Sprintf("%s %s%s in %s ", inst.cmdName, chownComment, srcHash, inst.dest)
	// logrus.Debug(commentStr)
	// logrus.Debug("perform Copy")
	// TODO: should this have been using origPaths instead of srcHash in the comment?
	runConfigWithCommentCmd := copyRunConfig(
		state.runConfig,
		withCmdCommentString(commentStr, state.operatingSystem))
	hit, err := b.probeCache(state, runConfigWithCommentCmd)
	if err != nil || hit {
		return err
	}

	imageMount, err := b.imageSources.Get(ctx, state.imageID, true, req.builder.platform)
	if err != nil {
		return errors.Wrapf(err, "failed to get destination image %q", state.imageID)
	}

	rwLayer, err := imageMount.NewRWLayer()
	if err != nil {
		return err
	}
	defer rwLayer.Release()

	destInfo, err := createDestInfo(state.runConfig.WorkingDir, inst, rwLayer, state.operatingSystem)
	if err != nil {
		return err
	}

	identity := b.idMapping.RootPair()
	// if a chown was requested, perform the steps to get the uid, gid
	// translated (if necessary because of user namespaces), and replace
	// the root pair with the chown pair for copy operations
	if inst.chownStr != "" {
		identity, err = parseChownFlag(ctx, b, state, inst.chownStr, destInfo.root, b.idMapping)
		if err != nil {
			if b.options.Platform != "windows" {
				return errors.Wrapf(err, "unable to convert uid/gid chown string to host mapping")
			}

			return errors.Wrapf(err, "unable to map container user account name to SID")
		}
	}

	for _, info := range inst.infos {
		opts := copyFileOptions{
			decompress: inst.allowLocalDecompression,
			archiver:   b.getArchiver(),
		}
		if !inst.preserveOwnership {
			opts.identity = &identity
		}
		if err := performCopyForInfo(destInfo, info, opts); err != nil {
			return errors.Wrapf(err, "failed to copy files")
		}
	}
	return b.exportImage(state, rwLayer, imageMount.Image(), runConfigWithCommentCmd)
}

func createDestInfo(workingDir string, inst copyInstruction, rwLayer builder.RWLayer, platform string) (copyInfo, error) {
	// Twiddle the destination when it's a relative path - meaning, make it
	// relative to the WORKINGDIR
	dest, err := normalizeDest(workingDir, inst.dest)
	if err != nil {
		return copyInfo{}, errors.Wrapf(err, "invalid %s", inst.cmdName)
	}

	return copyInfo{root: rwLayer.Root(), path: dest}, nil
}

// For backwards compat, if there's just one info then use it as the
// cache look-up string, otherwise hash 'em all into one
func getSourceHashFromInfos(infos []copyInfo) string {
	if len(infos) == 1 {
		return infos[0].hash
	}
	var hashs []string
	for _, info := range infos {
		hashs = append(hashs, info.hash)
	}
	return hashStringSlice("multi", hashs)
}

func hashStringSlice(prefix string, slice []string) string {
	hasher := sha256.New()
	hasher.Write([]byte(strings.Join(slice, ",")))
	return prefix + ":" + hex.EncodeToString(hasher.Sum(nil))
}

type runConfigModifier func(*container.Config)

func withCmd(cmd []string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = cmd
	}
}

func withArgsEscaped(argsEscaped bool) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.ArgsEscaped = argsEscaped
	}
}

// withCmdComment sets Cmd to a nop comment string. See withCmdCommentString for
// why there are two almost identical versions of this.
func withCmdComment(comment string, platform string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = append(getShell(runConfig, platform), "#(nop) ", comment)
	}
}

// withCmdCommentString exists to maintain compatibility with older versions.
// A few instructions (workdir, copy, add) used a nop comment that is a single arg
// where as all the other instructions used a two arg comment string. This
// function implements the single arg version.
func withCmdCommentString(comment string, platform string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = append(getShell(runConfig, platform), "#(nop) "+comment)
	}
}

func withEnv(env []string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Env = env
	}
}

// withEntrypointOverride sets an entrypoint on runConfig if the command is
// not empty. The entrypoint is left unmodified if command is empty.
//
// The dockerfile RUN instruction expect to run without an entrypoint
// so the runConfig entrypoint needs to be modified accordingly. ContainerCreate
// will change a []string{""} entrypoint to nil, so we probe the cache with the
// nil entrypoint.
func withEntrypointOverride(cmd []string, entrypoint []string) runConfigModifier {
	return func(runConfig *container.Config) {
		if len(cmd) > 0 {
			runConfig.Entrypoint = entrypoint
		}
	}
}

// withoutHealthcheck disables healthcheck.
//
// The dockerfile RUN instruction expect to run without healthcheck
// so the runConfig Healthcheck needs to be disabled.
func withoutHealthcheck() runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Healthcheck = &container.HealthConfig{
			Test: []string{"NONE"},
		}
	}
}

func copyRunConfig(runConfig *container.Config, modifiers ...runConfigModifier) *container.Config {
	copy := *runConfig
	copy.Cmd = copyStringSlice(runConfig.Cmd)
	copy.Env = copyStringSlice(runConfig.Env)
	copy.Entrypoint = copyStringSlice(runConfig.Entrypoint)
	copy.OnBuild = copyStringSlice(runConfig.OnBuild)
	copy.Shell = copyStringSlice(runConfig.Shell)

	if copy.Volumes != nil {
		copy.Volumes = make(map[string]struct{}, len(runConfig.Volumes))
		for k, v := range runConfig.Volumes {
			copy.Volumes[k] = v
		}
	}

	if copy.ExposedPorts != nil {
		copy.ExposedPorts = make(nat.PortSet, len(runConfig.ExposedPorts))
		for k, v := range runConfig.ExposedPorts {
			copy.ExposedPorts[k] = v
		}
	}

	if copy.Labels != nil {
		copy.Labels = make(map[string]string, len(runConfig.Labels))
		for k, v := range runConfig.Labels {
			copy.Labels[k] = v
		}
	}

	for _, modifier := range modifiers {
		modifier(&copy)
	}
	return &copy
}

func copyStringSlice(orig []string) []string {
	if orig == nil {
		return nil
	}
	return append([]string{}, orig...)
}

// getShell is a helper function which gets the right shell for prefixing the
// shell-form of RUN, ENTRYPOINT and CMD instructions
func getShell(c *container.Config, os string) []string {
	if 0 == len(c.Shell) {
		return append([]string{}, defaultShellForOS(os)[:]...)
	}
	return append([]string{}, c.Shell[:]...)
}

func (b *DepBuilder) probeCache(dispatchState *dispatchState, runConfig *container.Config) (bool, error) {
	// for i := 0; i < len(runConfig.Cmd); i++ {
	// 	logrus.Debug(runConfig.Cmd[i])
	// }
	//  || !strings.HasPrefix(runConfig.Cmd[2], "#(nop) ")
	// !(strings.Contains(runConfig.Cmd[2], "WORKDIR") || runConfig.Cmd[2] == "#(nop) ") && len(b.depLayers) > b.traceManager.LayerCount-1
	// if strings.Contains(runConfig.Cmd[2], "pip install numpy") {
	// 	logrus.Info(dispatchState)
	// 	return false, nil
	// }
	if !(strings.Contains(runConfig.Cmd[2], "WORKDIR") || runConfig.Cmd[2] == "#(nop) ") && len(b.depLayers) > b.traceManager.LayerCount-1 {
		logrus.Debug("check dep cache")
		depdencyLayer := []string{}
		for _, layerID := range b.depLayers[b.traceManager.LayerCount-1] {
			depdencyLayer = append(depdencyLayer, b.layersDigest[layerID])
			// logrus.Debugf("%d layer: %s", layerID, b.layersDigest[layerID])
		}
		// logrus.Debug(len(depdencyLayer))
		// logrus.Debug(b.baseImageDigest)
		if len(depdencyLayer) == 0 {
			depdencyLayer = append(depdencyLayer, b.baseImageDigest)
		}
		// if strings.Contains(runConfig.Cmd[2], "WORKDIR") || runConfig.Cmd[2] == "#(nop) " {
		// 	depdencyLayer = append(depdencyLayer, b.layersDigest[b.traceManager.LayerNum-1])
		// } else if len(depdencyLayer) == 0 {
		// 	depdencyLayer = append(depdencyLayer, b.baseImageDigest)
		// }
		depCachedID := b.docker.CheckLayer(runConfig, depdencyLayer, &b.layerList, &b.cacheIDList)
		if len(depCachedID) != 0 {
			logrus.Debugf("[Dep Cache] Hit : %s", runConfig.Cmd)
			logrus.Debugf("cache id is : %s", depCachedID)
			fmt.Fprint(b.Stdout, " ---> Using cache\n")
			dispatchState.imageID = depCachedID
			if b.firstBuild == true {
				var updateFiles []string
				var err error

				lastCacheID := b.cacheIDList[len(b.cacheIDList)-1]
				updateFiles, err = walkDir(dockerStorgePath+lastCacheID, updateFiles)

				logrus.Debugf("walk path : %s", dockerStorgePath+lastCacheID)

				if err == nil {
					for _, fi := range updateFiles {
						b.traceManager.fileUpdateRecord[strings.TrimPrefix(fi, dockerStorgePath+lastCacheID)] = b.traceManager.LayerNum
					}
				} else {
					return false, err
				}
			} else {
				b.newDepDigest = depdencyLayer
				b.newDepLayer = b.depLayers[b.traceManager.LayerCount-1]
			}
			return true, nil
		}
		logrus.Debug("[Dep Cache]Miss")
		// b.imageProber.Set()
		// err := b.imageProber.Set(dispatchState.imageID, runConfig)
		// logrus.Debug(depdencyLayer)
		// logrus.Debug(runConfig)
		return false, nil
	}
	// logrus.Debug("check cache")
	logrus.Info(dispatchState)
	cachedID, err := b.imageProber.Probe(dispatchState.imageID, runConfig)
	if cachedID == "" || err != nil {
		return false, err
	}
	logrus.Debugf("cache id is : %s", cachedID)
	fmt.Fprint(b.Stdout, " ---> Using cache\n")

	dispatchState.imageID = cachedID
	return true, nil
}

var defaultLogConfig = container.LogConfig{Type: "none"}

func (b *DepBuilder) probeAndCreate(ctx context.Context, dispatchState *dispatchState, runConfig *container.Config) (string, error) {
	if hit, err := b.probeCache(dispatchState, runConfig); err != nil || hit {
		return "", err
	}
	return b.create(ctx, runConfig)
}

func (b *DepBuilder) create(ctx context.Context, runConfig *container.Config) (string, error) {
	logrus.Debugf("[BUILDER] Command to be executed: %v", runConfig.Cmd)
	logrus.Debugf("[BUILDER] Image is: %v", runConfig.Image)

	hostConfig := hostConfigFromOptions(b.options)
	container, err := b.containerManager.Create(ctx, runConfig, hostConfig)
	if err != nil {
		return "", err
	}
	// TODO: could this be moved into containerManager.Create() ?
	for _, warning := range container.Warnings {
		fmt.Fprintf(b.Stdout, " ---> [Warning] %s\n", warning)
	}
	fmt.Fprintf(b.Stdout, " ---> Running in %s\n", stringid.TruncateID(container.ID))
	return container.ID, nil
}

func hostConfigFromOptions(options *types.ImageBuildOptions) *container.HostConfig {
	resources := container.Resources{
		CgroupParent: options.CgroupParent,
		CPUShares:    options.CPUShares,
		CPUPeriod:    options.CPUPeriod,
		CPUQuota:     options.CPUQuota,
		CpusetCpus:   options.CPUSetCPUs,
		CpusetMems:   options.CPUSetMems,
		Memory:       options.Memory,
		MemorySwap:   options.MemorySwap,
		Ulimits:      options.Ulimits,
	}

	hc := &container.HostConfig{
		SecurityOpt: options.SecurityOpt,
		Isolation:   options.Isolation,
		ShmSize:     options.ShmSize,
		Resources:   resources,
		NetworkMode: container.NetworkMode(options.NetworkMode),
		// Set a log config to override any default value set on the daemon
		LogConfig:  defaultLogConfig,
		ExtraHosts: options.ExtraHosts,
	}
	return hc
}
