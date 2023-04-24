package depbuilder // import "github.com/docker/docker/builder/depbuilder"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"bufio"
	"os"
	"strconv"
	"runtime/pprof"
	"crypto/rand"
	"math/big"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
)

var validCommitCommands = map[string]bool{
	"cmd":         true,
	"entrypoint":  true,
	"healthcheck": true,
	"env":         true,
	"expose":      true,
	"label":       true,
	"onbuild":     true,
	"stopsignal":  true,
	"user":        true,
	"volume":      true,
	"workdir":     true,
}

const (
	stepFormat = "Step %d/%d : %v"
)

type BuildManager struct {
	idMapping idtools.IdentityMapping
	backend   builder.Backend
	pathCache pathCache // TODO: make this persistent
}

func NewBuildManager(b builder.Backend, identityMapping idtools.IdentityMapping) (*BuildManager, error) {
	bm := &BuildManager{
		backend:   b,
		pathCache: &syncmap.Map{},
		idMapping: identityMapping,
	}
	return bm, nil
}

func (bm *BuildManager) Build(ctx context.Context, config backend.BuildConfig) (*builder.Result, error) {
	buildsTriggered.Inc()
	if config.Options.Dockerfile == "" {
		config.Options.Dockerfile = builder.DefaultDockerfileName
	}

	source, dockerfile, err := remotecontext.Detect(config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if source != nil {
			if err := source.Close(); err != nil {
				logrus.Debugf("[BUILDER] failed to remove temporary context: %v", err)
			}
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	builderOptions := builderOptions{
		Options:        config.Options,
		ProgressWriter: config.ProgressWriter,
		Backend:        bm.backend,
		PathCache:      bm.pathCache,
		IDMapping:      bm.idMapping,
	}
	b, err := newBuilder(ctx, builderOptions)
	if err != nil {
		return nil, err
	}
	return b.build(ctx, source, dockerfile)
}

type builderOptions struct {
	Options        *types.ImageBuildOptions
	Backend        builder.Backend
	ProgressWriter backend.ProgressWriter
	PathCache      pathCache
	IDMapping      idtools.IdentityMapping
}

// Builder is a Dockerfile builder
// It implements the builder.Backend interface.
type DepBuilder struct {
	options *types.ImageBuildOptions

	Stdout io.Writer
	Stderr io.Writer
	Aux    *streamformatter.AuxFormatter
	Output io.Writer

	docker builder.Backend

	idMapping        idtools.IdentityMapping
	disableCommit    bool
	imageSources     *imageSources
	pathCache        pathCache
	containerManager *containerManager
	imageProber      ImageProber
	platform         *specs.Platform

	firstBuild		bool
	depInfo 		*builder.DepInfo
	traceManager	*Tracee

	depLayers		[][]int
	layersDigest	map[int]string
	baseImageDigest string
	newDepLayer		[]int
	newDepDigest	[]string
	layerList		[]layer.DiffID
	cacheIDList		[]string
}

func newBuilder(ctx context.Context, options builderOptions) (*DepBuilder, error) {
	config := options.Options
	if config == nil {
		config = new(types.ImageBuildOptions)
	}

	imageProber, err := newImageProber(ctx, options.Backend, config.CacheFrom, config.NoCache)
	if err != nil {
		return nil, err
	}

	tm := &Tracee{
		traceLog:		 	nil,
		lastTime:		 	0,
		LayerDict:			make(map[int]string),
		fileUpdateRecord:	make(map[string]int),
	}

	b := &DepBuilder{
		options:          config,
		Stdout:           options.ProgressWriter.StdoutFormatter,
		Stderr:           options.ProgressWriter.StderrFormatter,
		Aux:              options.ProgressWriter.AuxFormatter,
		Output:           options.ProgressWriter.Output,
		docker:           options.Backend,
		idMapping:        options.IDMapping,
		imageSources:     newImageSources(options),
		pathCache:        options.PathCache,
		imageProber:      imageProber,
		containerManager: newContainerManager(options.Backend),
		firstBuild:		  false,
		depInfo:		  nil,
		traceManager:	  tm,
		depLayers:		  [][]int{},
		layersDigest:	  make(map[int]string),
		baseImageDigest:  "",
		newDepLayer:	  []int{},
		newDepDigest:	  []string{},
		layerList:		  []layer.DiffID{},
		cacheIDList:	  []string{},
	}

	// check dependency file
	logrus.Debug(config.Dockerfile)
	depInfo, err := b.docker.CheckBuildHistory(config.Dockerfile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if depInfo == nil {
		b.firstBuild = true
	}
	b.depInfo = depInfo

	// same as in Builder.Build in builder/builder-next/builder.go
	// TODO: remove once config.Platform is of type specs.Platform
	if config.Platform != "" {
		sp, err := platforms.Parse(config.Platform)
		if err != nil {
			return nil, err
		}
		b.platform = &sp
	}

	return b, nil
}

func buildLabelOptions(labels map[string]string, stages []instructions.Stage) {
	keys := []string{}
	for key := range labels {
		keys = append(keys, key)
	}

	// Sort the label to have a repeatable order
	sort.Strings(keys)
	for _, key := range keys {
		value := labels[key]
		stages[len(stages)-1].AddCommand(instructions.NewLabelCommand(key, value, true))
	}
}

func (b *DepBuilder) build(ctx context.Context, source builder.Source, dockerfile *parser.Result) (*builder.Result, error) {
	defer b.imageSources.Unmount()

	stages, metaArgs, err := instructions.Parse(dockerfile.AST)
	if err != nil {
		var uiErr *instructions.UnknownInstructionError
		if errors.As(err, &uiErr) {
			buildsFailed.WithValues(metricsUnknownInstructionError).Inc()
		}
		return nil, errdefs.InvalidParameter(err)
	}
	if b.options.Target != "" {
		targetIx, found := instructions.HasStage(stages, b.options.Target)
		if !found {
			buildsFailed.WithValues(metricsBuildTargetNotReachableError).Inc()
			return nil, errdefs.InvalidParameter(errors.Errorf("failed to reach build target %s in Dockerfile", b.options.Target))
		}
		stages = stages[:targetIx+1]
	}

	buildLabelOptions(b.options.Labels, stages)

	dockerfile.PrintWarnings(b.Stderr)

	dispatchState, err := b.dispatchDockerfileWithCancellation(ctx, stages, metaArgs, dockerfile.EscapeToken, source)
	if err != nil {
		return nil, err
	}
	if dispatchState.imageID == "" {
		buildsFailed.WithValues(metricsDockerfileEmptyError).Inc()
		return nil, errors.New("No image was generated. Is your Dockerfile empty?")
	}
	return &builder.Result{ImageID: dispatchState.imageID, FromImage: dispatchState.baseImage}, nil

}

func emitImageID(aux *streamformatter.AuxFormatter, state *dispatchState) error {
	if aux == nil || state.imageID == "" {
		return nil
	}
	return aux.Emit("", types.BuildResult{ID: state.imageID})
}

func processMetaArg(meta instructions.ArgCommand, shlex *shell.Lex, args *BuildArgs) error {
	// shell.Lex currently only support the concatenated string format
	envs := convertMapToEnvList(args.GetAllAllowed())
	if err := meta.Expand(func(word string) (string, error) {
		return shlex.ProcessWord(word, envs)
	}); err != nil {
		return err
	}
	for _, arg := range meta.Args {
		args.AddArg(arg.Key, arg.Value)
		args.AddMetaArg(arg.Key, arg.Value)
	}
	return nil
}

func printCommand(out io.Writer, currentCommandIndex int, totalCommands int, cmd interface{}) int {
	fmt.Fprintf(out, stepFormat, currentCommandIndex, totalCommands, cmd)
	fmt.Fprintln(out)
	return currentCommandIndex + 1
}

func transIntSlice(arr []string) []int {
	out := []int{}
	for _, v := range(arr) {
		iv, err := strconv.Atoi(v)
		if err == nil {
			out = append(out, iv)
		}
	}
	return out
}

func (b *DepBuilder) dispatchDockerfileWithCancellation(ctx context.Context, parseResult []instructions.Stage, metaArgs []instructions.ArgCommand, escapeToken rune, source builder.Source) (*dispatchState, error) {
	dispatchRequest := dispatchRequest{}
	buildArgs := NewBuildArgs(b.options.BuildArgs)
	totalCommands := len(metaArgs) + len(parseResult)
	currentCommandIndex := 1
	for _, stage := range parseResult {
		totalCommands += len(stage.Commands)
	}
	shlex := shell.NewLex(escapeToken)
	for i := range metaArgs {
		currentCommandIndex = printCommand(b.Stdout, currentCommandIndex, totalCommands, &metaArgs[i])

		err := processMetaArg(metaArgs[i], shlex, buildArgs)
		if err != nil {
			return nil, err
		}
	}

	stagesResults := newStagesBuildResults()
	// stageInd := map[string]int{}
	// logrus.Debug(totalCommands)

	// start tracee


	//get build history
	if b.firstBuild == false {
		depFileReader := bufio.NewScanner(b.depInfo.DepFile)
		dockerfileArchReader := bufio.NewScanner(b.depInfo.DockerfileArch)
		matchRow := 0
		for _, s := range parseResult {
			if !depFileReader.Scan() || !dockerfileArchReader.Scan() {
				break
			}
			matchRow++
			sourceCode := strings.TrimSpace(dockerfileArchReader.Text())
			depLine := strings.TrimSpace(depFileReader.Text())
			depSlice := transIntSlice(strings.Split(depLine, " "))
			if sourceCode == s.SourceCode {
				b.depLayers = append(b.depLayers, depSlice[1:])
			} else{
				b.depLayers = append(b.depLayers, []int{-1})
				logrus.Warning(sourceCode, s.SourceCode)
			}
			for _, cmd := range s.Commands {
				if !depFileReader.Scan() || !dockerfileArchReader.Scan() {
					break
				}
				matchRow++
				sourceCode := strings.TrimSpace(dockerfileArchReader.Text())
				depLine := strings.TrimSpace(depFileReader.Text())
				// logrus.Debug(depLine)
				depSlice := transIntSlice(strings.Split(depLine, " "))
				// logrus.Debug(depSlice)
				if sourceCode == fmt.Sprintf("%s", cmd){
					b.depLayers = append(b.depLayers, depSlice)
				} else {
					b.depLayers = append(b.depLayers, []int{-1})
					logrus.Warning(sourceCode, cmd)
				}
			}
		}
		// logrus.Debug(b.depLayers)
	}

	bd, err := b.docker.ApplyBuildHistory(b.options.Dockerfile)
	if err != nil {
		return nil, err
	}
	b.depInfo = bd

	// logrus.Debug(b.depInfo.DepFile, b.depInfo.DockerfileArch)

	//init writer
	dockerfileArchWriter := bufio.NewWriter(b.depInfo.DockerfileArch)
	defer b.depInfo.DockerfileArch.Close()
	depFileWriter := bufio.NewWriter(b.depInfo.DepFile)
	defer b.depInfo.DepFile.Close()

	//pref
	rid, _ := rand.Int(rand.Reader, big.NewInt(100000))
	pf, _ := os.Create(`/var/lib/docker/` + rid.String() + ".pprof")
	pprof.StartCPUProfile(pf)

	for _, s := range parseResult {
		stage := s
		if err := stagesResults.checkStageNameAvailable(stage.Name); err != nil {
			return nil, err
		}
		dispatchRequest = newDispatchRequest(b, escapeToken, source, buildArgs, stagesResults)

		// logrus.Debug(stage.BaseName)
		// logrus.Debug(fmt.Sprintf("%s\n", stage.SourceCode))
		dockerfileArchWriter.WriteString(fmt.Sprintf("%s\n", stage.SourceCode))
		depFileWriter.WriteString(fmt.Sprintf("-1\n"))

		currentCommandIndex = printCommand(b.Stdout, currentCommandIndex, totalCommands, stage.SourceCode)
		b.traceManager.CallTracee("Open")
		if err := initializeStage(ctx, dispatchRequest, &stage); err != nil {
			return nil, err
		}
		b.traceManager.CallTracee("Close")
		
		dispatchRequest.state.updateRunConfig()
		b.layersDigest[currentCommandIndex-1] = dispatchRequest.state.imageID
		b.baseImageDigest = dispatchRequest.state.imageID
		b.initLayerList(dispatchRequest.state.imageID)
		b.traceManager.LayerDict[currentCommandIndex-1] = dispatchRequest.state.imageID
		logrus.Debugf("the %d layer digest: %s", currentCommandIndex-1, dispatchRequest.state.imageID)
		fmt.Fprintf(b.Stdout, " ---> %s\n", stringid.TruncateID(dispatchRequest.state.imageID))
		for _, cmd := range stage.Commands {
			select {
			case <-ctx.Done():
				logrus.Debug("Builder: build cancelled!")
				fmt.Fprint(b.Stdout, "Build cancelled\n")
				buildsFailed.WithValues(metricsBuildCanceled).Inc()
				return nil, errors.New("Build cancelled")
			default:
				// Not cancelled yet, keep going...
			}

			// Start tracee to record
			// logrus.Debug(fmt.Sprintf("%s\n", cmd))
			
			dockerfileArchWriter.WriteString(fmt.Sprintf("%s\n", cmd))
			// logrus.Debug("Start tracee")
			currentCommandIndex = printCommand(b.Stdout, currentCommandIndex, totalCommands, cmd)
			
			// get layer id && check depdency trace
			b.traceManager.LayerCount = currentCommandIndex - 1
			b.traceManager.CallTracee("Open")
			b.traceManager.UpdateTime()
			if err := dispatch(ctx, dispatchRequest, cmd); err != nil {
				return nil, err
			}
			b.traceManager.CallTracee("Close")

			b.updateLayerList(dispatchRequest.state.imageID)

			b.layersDigest[currentCommandIndex-1] = dispatchRequest.state.imageID
			b.traceManager.LayerDict[currentCommandIndex-1] = dispatchRequest.state.imageID
			
			logrus.Debugf("the %d layer digest: %s", currentCommandIndex-1, dispatchRequest.state.imageID)
			
			for _,k := range b.newDepLayer{
				// logrus.Debugf("depStage is: %d", k)
				depFileWriter.WriteString(fmt.Sprintf("%d ", k))
			}
			depFileWriter.WriteString("\n")

			b.newDepLayer = b.newDepLayer[:0]
			b.newDepDigest = b.newDepDigest[:0]

			dispatchRequest.state.updateRunConfig()

			// logrus.Debug("check cacheID list: ", b.cacheIDList)

			fmt.Fprintf(b.Stdout, " ---> %s\n", stringid.TruncateID(dispatchRequest.state.imageID))
		}
		// End tracee and record
		// logrus.Debug("End tracee")

		dockerfileArchWriter.Flush()
		depFileWriter.Flush()

		cacheFile ,_ := os.OpenFile("/go/src/github.com/docker/docker/trans/buildcache.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		for _,v := range b.cacheIDList {
			fmt.Fprintf(cacheFile,"%s\n", v)
		}
		cacheFile.Close()

		b.cacheIDList = b.cacheIDList[:0]

		if err := emitImageID(b.Aux, dispatchRequest.state); err != nil {
			return nil, err
		}
		buildArgs.MergeReferencedArgs(dispatchRequest.state.buildArgs)
		if err := commitStage(dispatchRequest.state, stagesResults); err != nil {
			return nil, err
		}
	}

	pprof.StopCPUProfile()
	pf.Close()

	// cacheFile ,_ := os.Open("/go/src/github.com/docker/docker/trans/buildcache.log", "a+")

	// b.cacheIDList = b.cacheIDList[:0]

	buildArgs.WarnOnUnusedBuildArgs(b.Stdout)
	return dispatchRequest.state, nil
}

func (b *DepBuilder) checkBuildDepdency() bool {
	// hard coding
	lf, _ := os.Open("/tracee/tracee.log")
	b.traceManager.traceLog = lf

	stageID := b.docker.GetLastCacheID()
	if len(b.cacheIDList) == 0{
		content, err := os.ReadFile(`/var/lib/docker/aufs/layers/` + strings.TrimPrefix(stageID, "sha256:"))
		if err != nil {
			logrus.Debugf("fail get layers error: ", err)
		} else {
			ll := strings.Split(string(content), "\n")

			for i:=0; i < len(ll) - 1; i++ {
				b.cacheIDList = append(b.cacheIDList, ll[i])
			}
		}
	}
	b.traceManager.lastCacheID = stageID

	var err error
	flag := true
	b.newDepLayer, b.newDepDigest, err = b.traceManager.GetDepLayer()

	if err == nil {
		b.cacheIDList = append(b.cacheIDList, stageID)
	} else {
		flag = false
	}

	b.traceManager.traceLog.Close()

	logrus.Debugf("get cache id %s", stageID)
	return flag
}

func (b *DepBuilder) initLayerList(layerDigest string) {
	imageFile := `/var/lib/docker/image/aufs/imagedb/content/sha256/` + strings.TrimPrefix(layerDigest, "sha256:")
	content, err := os.ReadFile(imageFile)
	if err != nil {
		logrus.Error(err)
	}
	im, err := image.NewFromJSON(content)
	if err != nil {
		logrus.Error(err)
		return
	}
	b.layerList = im.RootFS.DiffIDs

	b.cacheIDList = b.cacheIDList[:0]
}

func (b *DepBuilder) updateLayerList(layerDigest string) {
	imageFile := `/var/lib/docker/image/aufs/imagedb/content/sha256/` + strings.TrimPrefix(layerDigest, "sha256:")
	content, err := os.ReadFile(imageFile)
	if err != nil {
		logrus.Error(err)
	}
	im, err := image.NewFromJSON(content)
	if err != nil {
		logrus.Error(err)
		return
	}
	if len(b.layerList) > len(im.RootFS.DiffIDs){
		logrus.Error("layerlist more than diffid")
	}
	for i:= len(b.layerList); i < len(im.RootFS.DiffIDs); i++ {
		b.layerList = append(b.layerList, im.RootFS.DiffIDs[i])
	}
}

func BuildFromConfig(ctx context.Context, config *container.Config, changes []string, os string) (*container.Config, error) {
	if len(changes) == 0 {
		return config, nil
	}

	dockerfile, err := parser.Parse(bytes.NewBufferString(strings.Join(changes, "\n")))
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	b, err := newBuilder(ctx, builderOptions{
		Options: &types.ImageBuildOptions{NoCache: true},
	})
	if err != nil {
		return nil, err
	}

	// ensure that the commands are valid
	for _, n := range dockerfile.AST.Children {
		if !validCommitCommands[strings.ToLower(n.Value)] {
			return nil, errdefs.InvalidParameter(errors.Errorf("%s is not a valid change command", n.Value))
		}
	}

	b.Stdout = io.Discard
	b.Stderr = io.Discard
	b.disableCommit = true

	var commands []instructions.Command
	for _, n := range dockerfile.AST.Children {
		cmd, err := instructions.ParseCommand(n)
		if err != nil {
			return nil, errdefs.InvalidParameter(err)
		}
		commands = append(commands, cmd)
	}

	dispatchRequest := newDispatchRequest(b, dockerfile.EscapeToken, nil, NewBuildArgs(b.options.BuildArgs), newStagesBuildResults())
	// We make mutations to the configuration, ensure we have a copy
	dispatchRequest.state.runConfig = copyRunConfig(config)
	dispatchRequest.state.imageID = config.Image
	dispatchRequest.state.operatingSystem = os
	for _, cmd := range commands {
		err := dispatch(ctx, dispatchRequest, cmd)
		if err != nil {
			return nil, errdefs.InvalidParameter(err)
		}
		dispatchRequest.state.updateRunConfig()
	}

	return dispatchRequest.state.runConfig, nil
}

func convertMapToEnvList(m map[string]string) []string {
	result := []string{}
	for k, v := range m {
		result = append(result, k+"="+v)
	}
	return result
}
