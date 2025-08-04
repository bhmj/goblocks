package containermanager

/*
	Container manager is application specific and has its built-in limits and rules.

	1) Home directory is always mounted as /home/dummy, no matter what user is set as default in the image.
	2) Non-root user "dummy" is recommended as a default user. See Dockerfile.golang for an example.
	3) Cargo files should be written to {home directory}
	4) Container working directory is {home directory} at container start.

	It is possible to run container as root (see TestRunner) but only considering the above statements.
*/

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bhmj/goblocks/file"
	"github.com/bhmj/goblocks/log"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"
	dockermount "github.com/docker/docker/api/types/mount"
	dockernetwork "github.com/docker/docker/api/types/network"
	dockervolume "github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
)

var (
	statsPeriod              = 200 * time.Millisecond
	errContainerLimitCPU     = errors.New("CPU limit exceeded")
	errContainerLimitNet     = errors.New("network limit exceeded")
	errContainerLimitTime    = errors.New("run time limit exceeded")
	ErrContainerCreate       = errors.New("container create error")
	ErrContainerStart        = errors.New("container start error")
	ErrContainerReady        = errors.New("ready wait timeout")
	ErrContainerBusy         = errors.New("container is already in use")
	ErrContainerDoesNotExist = errors.New("container does not exist")
	ErrStdoutChannelNotSet   = errors.New("stdout channel is not set")
)

// Resources defines a set of resources (or limits, for that matter) which are available from within the container
type Resources struct {
	RAM    uint // Mb
	CPUs   uint // CPUs or cores (in 1/1000)
	Net    bool // connect to network
	TmpDir uint // Mb
}

// RuntimeLimits defines a single run limits for the exec
type RuntimeLimits struct {
	CPUTime uint // msec
	Net     uint // bytes
	RunTime uint // sec
	TmpDir  uint // Mb
}

const defaultReadyTimeout = 4 * time.Second

// ContainerSetup defines the image and its settings to run the container
type ContainerSetup struct {
	Image            string
	DefaultCmd       bool          // true for running the container with its own built-in Docker CMD
	ReadyString      string        // set a substring to look for in container logs which signals that the container is ready
	ReadyTimeout     time.Duration // timeout for looking for ReadyString in container logs. Default is `defaultReadyTimeout`
	WorkingDir       string        // absolute host directory mounted as /home/dummy/
	WorkingDirRO     bool
	CacheVolume      []string
	CacheVolumeMount []string
	Envs             map[string]string
	Label            string // {compiler|runner}-{lang}-{version}
	Resources
}

// ContainerPipe defines a channel for reading data from container's stdout/stderr and, optionally, write data into container's stdin
type ContainerPipe struct {
	StdIn    chan []byte
	StdOut   chan []byte
	StdErr   chan []byte
	Consumed chan ConsumedResources // channel is written by executor after the streaming has stopped or the container is stopped
}

type ConsumedResources struct {
	CPUTime uint64 // mCPU/sec
	Net     uint64 // bytes
}

// ContainerManager defines a container manager interface
type ContainerManager interface {
	ImageExist(name string) error
	FindContainers(name string) ([]string, error)
	ContainerExist(containerID string) bool
	WaitForIdle(containerID string, timeout time.Duration) error
	CreateAndRunContainer(setup *ContainerSetup) (string, error)
	StopContainer(containerID string, force bool)
	Execute(containerID string, commands []string, pipe ContainerPipe, limits RuntimeLimits) (int, error) // TODO: return consumed resources!
	Stats() map[string]string
}

type containerState int

const (
	containerStateIdle = 0
	containerStateBusy = 1
)

type containerManager struct {
	sync.RWMutex
	cli        *dockerclient.Client
	containers map[string]containerState // key: container ID
	logger     log.MetaLogger
}

// New returns an instance of the container manager
func New(logger log.MetaLogger) (ContainerManager, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &containerManager{
		cli:        cli,
		containers: make(map[string]containerState),
		logger:     logger,
	}, nil
}

// Stats returns a map of running containers
func (cm *containerManager) Stats() map[string]string {
	cm.RLock()
	defer cm.RUnlock()
	result := make(map[string]string, len(cm.containers))
	for id, st := range cm.containers {
		state := "idle"
		if st == containerStateBusy {
			state = "busy"
		}
		result[id] = state
	}
	return result
}

// FindContainers returns container IDs of the running containers matching tag
func (cm *containerManager) FindContainers(name string) ([]string, error) {
	var result []string
	re := regexp.MustCompile(name)
	cs, err := cm.cli.ContainerList(context.Background(), dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		for _, nm := range c.Names {
			nm = strings.Replace(nm, "/", "", 1)
			if re.MatchString(nm) {
				result = append(result, c.ID)
				break
			}
		}
	}
	return result, nil
}

// ImageExist returns true if image is available on local host
func (cm *containerManager) ImageExist(image string) error {
	ims, err := cm.cli.ImageList(context.Background(), dockerimage.ListOptions{})
	if err != nil {
		return err
	}
	for _, im := range ims {
		for _, tag := range im.RepoTags {
			if tag == image {
				return nil
			}
		}
	}
	return fmt.Errorf("docker image not found, consider creating or pulling: %s", image)
}

// containerExists returns true if container exists in registry.
func (cm *containerManager) ContainerExist(ID string) bool {
	cm.RLock()
	defer cm.RUnlock()
	if _, found := cm.containers[ID]; found {
		return true
	}
	return false
}

// registerContainer registers a container info in registry.
func (cm *containerManager) registerContainer(containerID string) {
	cm.Lock()
	defer cm.Unlock()
	cm.containers[containerID] = containerStateIdle
}

// unregisterContainer removes a container info from registry.
func (cm *containerManager) unregisterContainer(ID string) {
	cm.Lock()
	defer cm.Unlock()
	delete(cm.containers, ID)
}

// CreateAndRunContainer creates and runs the container in sleep mode. Returns ID of a confirmed running container.
func (cm *containerManager) CreateAndRunContainer(setup *ContainerSetup) (string, error) {
	var mounts []dockermount.Mount

	// create working dir if not exists
	err := file.Mkdir(setup.WorkingDir)
	if err != nil {
		dir, _ := os.Getwd()
		cm.logger.Error("create working dir", log.String("in", dir), log.String("dir", setup.WorkingDir), log.Error(err))
		return "", err
	}
	// mount home dir
	mounts = append(mounts, dockermount.Mount{
		Type:     dockermount.TypeBind,
		ReadOnly: setup.WorkingDirRO, // true for runner, false for compiler
		Source:   setup.WorkingDir,
		Target:   "/home/dummy/",
	})
	if setup.TmpDir > 0 {
		// mount temp dir
		mounts = append(mounts, dockermount.Mount{
			Type: dockermount.TypeTmpfs,
			TmpfsOptions: &dockermount.TmpfsOptions{
				SizeBytes: int64(setup.TmpDir) * 1024 * 1024, // Mb to bytes
			},
			Target: "/tmp/",
		})
	}

	// mount cache/dependency volumes
	for i := range setup.CacheVolume {
		volumeName := setup.CacheVolume[i]
		mountPoint := setup.CacheVolumeMount[i]
		err := cm.ensureVolume(volumeName)
		if err != nil {
			return "", fmt.Errorf("ensure volume: %w", err)
		}
		mounts = append(mounts, dockermount.Mount{
			Type:   dockermount.TypeVolume,
			Source: volumeName,
			Target: mountPoint,
		})
	}

	// default command for containers that do not provide idle mode
	commands := []string{"sh", "-c", "trap 'exit 0' TERM INT; while :; do sleep 0.5; wait || true; done"}

	if setup.DefaultCmd {
		commands = nil
	}

	// Define container configuration
	config := dockercontainer.Config{
		Image:           setup.Image,
		Cmd:             commands, //
		Tty:             true,     // Allocate a pseudo-TTY
		WorkingDir:      "/home/dummy/",
		NetworkDisabled: !setup.Net,
	}

	netMode := dockernetwork.NetworkNone
	if setup.Net {
		netMode = dockernetwork.NetworkBridge
	}
	hostConfig := dockercontainer.HostConfig{
		AutoRemove:     true,              // container removes itself after process exited (usually at unsuccessful start)
		ReadonlyRootfs: !setup.DefaultCmd, // FIXME: RW only needed for DOSASM
		Mounts:         mounts,
		NetworkMode:    dockercontainer.NetworkMode(netMode),
		Resources: dockercontainer.Resources{
			NanoCPUs:   int64(setup.CPUs) * 1000000,    // mCPUs to nCPUs
			Memory:     int64(setup.RAM) * 1024 * 1024, // Mb
			MemorySwap: int64(setup.RAM) * 1024 * 1024, // Mb
		},
	}

	ctx := context.Background()

	// Create the container
	resp, err := cm.cli.ContainerCreate(ctx, &config, &hostConfig, nil, nil, setup.Label)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrContainerCreate, err)
	}

	// Start the container
	err = cm.cli.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{})
	if err != nil {
		go cm.StopContainer(resp.ID, false)
		return "", fmt.Errorf("%w: %w", ErrContainerStart, err)
	}

	// Wait until ready
	for {
		time.Sleep(20 * time.Millisecond)
		// TODO: timeout
		containerState, err := cm.cli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			cm.logger.Error("wait-ready", log.Error(err))
			go cm.StopContainer(resp.ID, false)
			return "", fmt.Errorf("inspect container: %w", err)
		}
		if containerState.State.Status == "running" {
			break
		}
	}

	if setup.ReadyString != "" {
		timeout := defaultReadyTimeout
		if setup.ReadyTimeout > 0 {
			timeout = setup.ReadyTimeout
		}
		if cm.grepLogs(resp.ID, setup.ReadyString, timeout) != nil {
			cm.logger.Error("wait-ready-string")
			go cm.StopContainer(resp.ID, false)
			return "", ErrContainerReady
		}
	}

	cm.registerContainer(resp.ID)

	return resp.ID, nil
}

// StopContainer stops and removes the running (or sleeping) container.
func (cm *containerManager) StopContainer(containerID string, force bool) {
	if !cm.ContainerExist(containerID) && !force {
		return
	}
	gracefulTimeout := 2
	err := cm.cli.ContainerStop(context.Background(), containerID, dockercontainer.StopOptions{Timeout: &gracefulTimeout})
	if err != nil {
		cm.logger.Warn("container stop", log.Error(err))
		return
	}
	err = cm.cli.ContainerRemove(context.Background(), containerID, dockercontainer.RemoveOptions{})
	if err != nil {
		cm.logger.Warn("container remove", log.Error(err))
		return
	}
	cm.unregisterContainer(containerID)
}

// Execute runs specified command(s) inside a running container and waits for end of the process OR stops the container if the limits were exceeded.
// Returns when the container has done executing. Returns app exit code and/or error.
func (cm *containerManager) Execute(containerID string, commands []string, pipe ContainerPipe, limits RuntimeLimits) (int, error) {
	if pipe.StdOut == nil {
		return 0, ErrStdoutChannelNotSet
	}
	if pipe.StdErr == nil {
		pipe.StdErr = pipe.StdOut // use stdout for stderr if not set
	}
	execID, err := cm.createExecutor(containerID, commands)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "No such container") {
			cm.unregisterContainer(containerID)
			return 0, fmt.Errorf("%w: %w", ErrContainerDoesNotExist, err)
		}
		return 0, fmt.Errorf("create executor: %w", err)
	}

	return cm.execAttach(containerID, execID, pipe, limits)
}

// getCurrentStats returns current CPU and network metric
func (cm *containerManager) getCurrentStats(containerID string) (uint64, uint64, error) {
	stats, err := cm.cli.ContainerStatsOneShot(context.Background(), containerID)
	if err != nil {
		return 0, 0, fmt.Errorf("ContainerStats: %v", err)
	}
	var containerStats dockercontainer.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&containerStats); err != nil {
		return 0, 0, fmt.Errorf("decode stats: %v", err)
	}
	startCPU := containerStats.CPUStats.CPUUsage.TotalUsage
	startNet := containerStats.Networks["eth0"].TxBytes + containerStats.Networks["eth0"].RxBytes
	return startCPU, startNet, nil
}

func (cm *containerManager) setBusy(containerID string) error {
	cm.Lock()
	defer cm.Unlock()
	if cm.containers[containerID] == containerStateBusy {
		return ErrContainerBusy
	}
	cm.containers[containerID] = containerStateBusy

	return nil
}

func (cm *containerManager) setIdle(containerID string) {
	cm.Lock()
	defer cm.Unlock()
	_, found := cm.containers[containerID]
	if !found {
		return
	}
	cm.containers[containerID] = containerStateIdle
}

func (cm *containerManager) WaitForIdle(containerID string, timeout time.Duration) error {
	start := time.Now()
	for {
		cm.RLock()
		state, found := cm.containers[containerID]
		cm.RUnlock()
		if !found {
			return fmt.Errorf("container not found")
		}
		if state == containerStateIdle {
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for container to be idle")
		}
	}
}

// createExecutor prepares docker execution environment.
func (cm *containerManager) createExecutor(containerID string, commands []string) (string, error) {
	execConfig := dockercontainer.ExecOptions{
		Cmd:          commands,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	err := cm.setBusy(containerID)
	if err != nil {
		return "", err
	}

	// ContainerExecCreate creates process but does not start it
	execResp, err := cm.cli.ContainerExecCreate(context.Background(), containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("ContainerExecCreate: %v", err) // "container not found" is handled by caller
	}
	return execResp.ID, nil
}

// execAttach connects to an exec process and controls process flow. This is a synchronous function which means
// that on exit the exec process is either successfully stopped or terminating.
// Returns exit code + error.
func (cm *containerManager) execAttach(containerID, execID string, pipe ContainerPipe, limits RuntimeLimits) (int, error) {
	// ContainerExecAttach actually starts execution
	execAttachResp, err := cm.cli.ContainerExecAttach(context.Background(), execID, dockercontainer.ExecStartOptions{})
	if err != nil {
		return 0, fmt.Errorf("ContainerExecAttach: %v", err)
	}
	defer execAttachResp.Close()

	// stats ticker
	ticker := time.NewTicker(statsPeriod)
	startTime := time.Now()
	startCPU, startNet, err := cm.getCurrentStats(containerID) // ns, bytes
	if err != nil {
		return 0, fmt.Errorf("get zero state: %w", err)
	}

	// start streaming output
	doneStreaming := make(chan struct{})
	stopStreaming := make(chan struct{})
	go cm.streamOutput(doneStreaming, stopStreaming, pipe.StdIn, pipe.StdOut, pipe.StdErr, execAttachResp.Reader)

	var curCPU, curNet uint64
outer:
	for {
		select {
		case <-doneStreaming:
			break outer
		case <-ticker.C:
			// check stats
			curCPU, curNet, err = cm.checkLimits(containerID, startTime, startCPU, startNet, limits) // ns, bytes
			if err != nil {
				select {
				case <-doneStreaming:
					break outer
				default:
					close(stopStreaming)
				}
				go cm.StopContainer(containerID, false)
				pipe.Consumed <- ConsumedResources{CPUTime: (curCPU - startCPU) / 1000000, Net: curNet - startNet} // ns -> msec, bytes
				return 301, err
			}
		}
	}

	curCPU, curNet, _ = cm.checkLimits(containerID, startTime, startCPU, startNet, limits)
	pipe.Consumed <- ConsumedResources{CPUTime: (curCPU - startCPU) / 1000000, Net: curNet - startNet} // ns -> msec, bytes

	// Wait for the exec instance to finish (TODO: loop waiting!)
	resp, err := cm.cli.ContainerExecInspect(context.Background(), execID)
	if err != nil {
		cm.logger.Error("ContainerExecInspect", log.Error(err))
		return 300, err
	}

	go cm.teardown(containerID)

	return resp.ExitCode, nil
}

func (cm *containerManager) grepLogs(containerID string, needle string, timeout time.Duration) error {
	logs, err := cm.cli.ContainerLogs(context.Background(), containerID, dockercontainer.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		panic(err)
	}
	defer logs.Close()

	found := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("Log:", line)
			if strings.Contains(line, needle) {
				found <- true
				return
			}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			fmt.Println("Error reading logs:", err)
		}
	}()

	select {
	case <-found:
		return nil
	case <-time.After(timeout):
		return errors.New("timeout waiting for log output")
	}
}

func (cm *containerManager) teardown(containerID string) {
	cm.killAll(containerID) // kill spawned processes
	cm.setIdle(containerID)
}

func (cm *containerManager) killAll(containerID string) {
	execConfig := dockercontainer.ExecOptions{
		User: "root",
		Cmd:  []string{"kill", "--", "-1"},
	}
	execResp, err := cm.cli.ContainerExecCreate(context.Background(), containerID, execConfig)
	if err != nil {
		return
	}

	err = cm.cli.ContainerExecStart(context.Background(), execResp.ID, dockercontainer.ExecStartOptions{})
	if err != nil {
		return
	}

	for done := false; !done; {
		resp, err := cm.cli.ContainerExecInspect(context.Background(), execResp.ID)
		if err != nil {
			return
		}
		if !resp.Running {
			done = true
		} else {
			time.Sleep(50 * time.Millisecond) //nolint:gomnd
		}
	}
}

// checkLimits checks the running container stats and returns error if some resource is exhausted.
// Returns current CPU time (nsec) and network traffic (bytes).
func (cm *containerManager) checkLimits(containerID string, startTime time.Time, startCPU uint64, startNet uint64, limits RuntimeLimits) (uint64, uint64, error) {
	statsResponse, err := cm.cli.ContainerStatsOneShot(context.Background(), containerID)
	if err != nil {
		cm.logger.Error("ContainerStatsOneShot", log.Error(err))
		return 0, 0, nil
	}
	defer statsResponse.Body.Close()
	var stats dockercontainer.StatsResponse
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		cm.logger.Error("startResponse decode", log.Error(err))
		return 0, 0, nil
	}

	currentCPU := stats.CPUStats.CPUUsage.TotalUsage                              // ns
	currentNet := stats.Networks["eth0"].RxBytes + stats.Networks["eth0"].TxBytes // bytes
	timeElapsed := time.Since(startTime)

	if (currentCPU-startCPU)/1000000 > uint64(limits.CPUTime) { // convert consumed to msec
		return currentCPU, currentNet, errContainerLimitCPU
	}
	if (currentNet-startNet)/1024/1024 > uint64(limits.Net) {
		return currentCPU, currentNet, errContainerLimitNet
	}
	if timeElapsed > time.Duration(limits.RunTime)*time.Second {
		return currentCPU, currentNet, fmt.Errorf("%w: elapsed=%v, limit=%v", errContainerLimitTime, timeElapsed, time.Duration(limits.RunTime)*time.Second)
	}
	return currentCPU, currentNet, nil
}

// ensureVolume finds or creates a named Docker volume.
func (cm *containerManager) ensureVolume(name string) error {
	_, err := cm.cli.VolumeInspect(context.Background(), name)
	if err == nil {
		// volume exists
		return nil
	}
	if dockerclient.IsErrNotFound(err) {
		// create volume
		_, err = cm.cli.VolumeCreate(context.Background(), dockervolume.CreateOptions{Name: name})
		if err != nil {
			return fmt.Errorf("volume create: %w", err)
		}
		return nil
	}
	return fmt.Errorf("volume inspect: %w", err)
}

// streamOutput reads data from buffered IO reader and forwards it into [stdoutCh] and [stderrCh]. Closes [done] on finish.
func (cm *containerManager) streamOutput(
	doneCh chan struct{},
	stopCh chan struct{},
	_ /*stdinCh*/ chan []byte,
	stdoutCh chan []byte,
	stderrCh chan []byte,
	r *bufio.Reader,
) {
	defer close(doneCh)

	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	header := make([]byte, 8)
	buf := make([]byte, 120*1024) // 6-bit divisible (base64)
	for {
		// first read frame header (8 bytes)
		n, err := io.ReadFull(r, header)
		if n < 8 || err != nil {
			if err != io.EOF {
				cm.logger.Error("reading stream header", log.Error(err))
			}
			return // EOF: transmission finished
		}
		// get payload type and size
		streamType := int(header[0])
		dataLen := int(binary.BigEndian.Uint32(header[4:8]))
		// read frame into buf (possibly in parts) and flush it into corresponding channel
		n = 0
		for i := 0; i < dataLen; i += n {
			buf = buf[:min(cap(buf), dataLen-i)] // no more than buf capacity
			n, err = r.Read(buf)
			if err != nil {
				if err != io.EOF {
					cm.logger.Error("reading stream", log.Error(err))
				}
				return // EOF: transmission finished
			}
			var ch chan []byte
			switch streamType {
			case 1: // stdout
				ch = stdoutCh
			case 2: // stderr
				ch = stderrCh
			}
			b := append([]byte{}, buf[:n]...)
			select {
			case <-stopCh:
				return
			default:
				ch <- b
			}
		}
	}
}
