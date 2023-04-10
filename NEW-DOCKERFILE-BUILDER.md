## New Dockerfile Builder

Use EBPF tools(eg. tracee) to capture filesystem dependency.

Share layers with filesystem dependency.

docker run --rm --privileged  -e DOCKER_CROSSPLATFORMS -e BUILD_APT_MIRROR -e BUILDFLAGS -e KEEPBUNDLE -e DOCKER_BUILD_ARGS -e DOCKER_BUILD_GOGC -e DOCKER_BUILD_OPTS -e DOCKER_BUILD_PKGS -e DOCKER_BUILDKIT -e DOCKER_BASH_COMPLETION_PATH -e DOCKER_CLI_PATH -e DOCKER_DEBUG -e DOCKER_EXPERIMENTAL -e DOCKER_GITCOMMIT -e DOCKER_GRAPHDRIVER -e DOCKER_LDFLAGS -e DOCKER_PORT -e DOCKER_REMAP_ROOT -e DOCKER_ROOTLESS -e DOCKER_STORAGE_OPTS -e DOCKER_TEST_HOST -e DOCKER_USERLANDPROXY -e DOCKERD_ARGS -e DELVE_PORT -e GITHUB_ACTIONS -e TEST_FORCE_VALIDATE -e TEST_INTEGRATION_DIR -e TEST_INTEGRATION_USE_SNAPSHOTTER -e TEST_SKIP_INTEGRATION -e TEST_SKIP_INTEGRATION_CLI -e TESTCOVERAGE -e TESTDEBUG -e TESTDIRS -e TESTFLAGS -e TESTFLAGS_INTEGRATION -e TESTFLAGS_INTEGRATION_CLI -e TEST_FILTER -e TIMEOUT -e VALIDATE_REPO -e VALIDATE_BRANCH -e VALIDATE_ORIGIN_BRANCH -e VERSION -e PLATFORM -e DEFAULT_PRODUCT_LICENSE -e PRODUCT -e PACKAGER_NAME -v "/media/copyright/0887C86A5FC2749C/code/moby/.:/go/src/github.com/docker/docker/." -v "/media/copyright/0887C86A5FC2749C/code/moby/.git:/go/src/github.com/docker/docker/.git" -v docker-dev-cache:/root/.cache -v docker-mod-cache:/go/pkg/mod/ -v /home/copyright/tracee_test:/tracee    -t -i "docker-dev" bash

hack/make.sh binary install-binary run

docker run  --name tracee --rm -it  --pid=host --cgroupns=host --privileged  -v /etc/os-release:/etc/os-release-host:ro -v /home/copyright/tracee_test:/tmp  -e LIBBPFGO_OSRELEASE_FILE=/etc/os-release-host aquasec/tracee:0.9.3 trace --trace container --trace event=security_file_open,open* --trace open.pathname!="/var/run/docker/runtime-runc/*" --trace openat.pathname!="/var/run/docker/runtime-runc/*" --trace security_file_open.pathname!="/var/run/docker/runtime-runc/*" --output out-file:/tmp/tracee.log --output json


FROM alpine
RUN mkdir /a
RUN mkdir /b
WORKDIR /a
RUN echo "test" > test.txt
RUN echo "test2" > test2.txt
RUN echo "test3" >> test2.txt
WORKDIR /b
RUN echo "test" > test.txt
ENTRYPOINT [echo, "hello world!"]