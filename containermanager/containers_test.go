package containermanager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bhmj/goblocks/file"
	"github.com/bhmj/goblocks/log"
	"github.com/stretchr/testify/assert"
)

/*
	Consider building a test image befor running tests:

	$ docker build -t golang:dummy -f Dockerfile.golang .
*/

const playgroundRoot string = "./tmp/"

var workingDir string

func TestMain(m *testing.M) {
	file.Rmdir(playgroundRoot)
	dir, _ := os.Getwd()
	workingDir = filepath.Join(dir, "tmp/home")
	file.Mkdir(workingDir)
	m.Run()
	// teardown
	file.Rmdir(playgroundRoot)
}

func streamReader(t *testing.T, pipe ContainerPipe) {
	stdout := ""
	stderr := ""
	for pipe.StdOut != nil || pipe.StdErr != nil {
		select {
		case bytes, ok := <-pipe.StdOut:
			if !ok {
				pipe.StdOut = nil
			} else {
				stdout += string(bytes)
			}
		case bytes, ok := <-pipe.StdErr:
			if !ok {
				pipe.StdErr = nil
			} else {
				stderr += string(bytes)
			}
		case consumed, ok := <-pipe.Consumed:
			if !ok {
				pipe.StdErr = nil
			} else {
				stdout += fmt.Sprintf("\nCONSUMED: %d mCPU, %d traffic\n", consumed.CPUTime, consumed.Net)
			}
		}
	}
	t.Logf("STDOUT: %s\n", stdout)
	t.Logf("STDERR: %s\n", stderr)
}

// TestRunner:
// - create the container
// - execute a simple SH script
// - stream the output to caller
// - stop the container
func TestRunner(t *testing.T) {
	a := assert.New(t)

	logger, err := log.New("debug", false)
	a.NoError(err)

	file.Mkdir(playgroundRoot) // create if not exists
	cm, err := New(logger)
	a.NoError(err)

	setup := ContainerSetup{
		Image:      "alpine:latest",
		WorkingDir: workingDir,
		Label:      "runner-alpine-0.0",
		Resources: Resources{
			RAM:  256,
			CPUs: 1000,
			Net:  true,
		},
	}

	t0 := time.Now()

	ID, err := cm.CreateAndRunContainer(&setup)
	a.NoError(err)

	t.Logf("Container created in %v\n", time.Since(t0))

	// write cargo
	_ = os.WriteFile(filepath.Join(playgroundRoot, "home", "main.sh"), []byte("echo ''\necho 'hi there'\necho 'bye now'"), 0755)

	pipe := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	limits := RuntimeLimits{
		CPUTime: 10000,
		Net:     10,
		RunTime: 50,
	}

	go streamReader(t, pipe)

	t0 = time.Now()
	commands := []string{"sh", "-c", "/home/dummy/main.sh"}
	code, err := cm.Execute(ID, commands, pipe, limits)
	t.Logf("Executed in %v\n", time.Since(t0))

	close(pipe.StdIn)
	close(pipe.StdOut)
	close(pipe.StdErr)

	a.NoError(err)
	a.Equal(code, 0)

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Stopped in %v\n", time.Since(t0))
}

// TestCompiler:
// - create Golang container
// - load a .go file in a mounted dir
// - run the compiler, stream the output to caller
// - stop the Golang container
// - create runner container
// - run the binary in runner, stream output to caller
// - stop runner container
func TestCompiler(t *testing.T) {
	a := assert.New(t)

	logger, err := log.New("debug", false)
	a.NoError(err)

	cm, err := New(logger)
	a.NoError(err)

	setup := ContainerSetup{
		Image:      "golang:dummy",
		WorkingDir: workingDir,
		Label:      "compiler-golang-0.0",
		Envs:       map[string]string{"CGO_ENABLED": "0"},
		Resources: Resources{
			RAM:    256,
			CPUs:   1000,
			Net:    true,
			TmpDir: 40, // compiler needs some temp disk space
		},
	}

	t0 := time.Now()

	ID, err := cm.CreateAndRunContainer(&setup)
	a.NoError(err)

	t.Logf("Compiler created in %v\n", time.Since(t0))

	// write cargo
	_ = os.WriteFile(filepath.Join(playgroundRoot, "home", "main.go"), []byte(strTestCompiler), 0755)

	pipe := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	limits := RuntimeLimits{
		CPUTime: 10000,
		Net:     10,
		RunTime: 50,
	}

	go streamReader(t, pipe)

	t0 = time.Now()
	commands := []string{"go", "build", "-trimpath", "-o", "main", "main.go"}
	code, err := cm.Execute(ID, commands, pipe, limits)
	t.Logf("Compiler executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Compiler stopped in %v\n", time.Since(t0))

	close(pipe.StdIn)
	close(pipe.StdOut)
	close(pipe.StdErr)

	// RUN

	setup = ContainerSetup{
		Image:      "alpine:latest",
		WorkingDir: workingDir, // in real world scenario we may want to move the binary from compiler home dir into runner home dir
		Label:      "runner-golang-0.0",
		Resources: Resources{
			RAM:  256,
			CPUs: 1000,
			Net:  true,
		},
	}

	t0 = time.Now()

	ID, err = cm.CreateAndRunContainer(&setup)
	a.NoError(err)

	t.Logf("Runner created in %v\n", time.Since(t0))

	pipe2 := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	go streamReader(t, pipe2)

	t0 = time.Now()
	commands = []string{"./main"}
	code, err = cm.Execute(ID, commands, pipe2, limits)
	t.Logf("Runner executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Runner stopped in %v\n", time.Since(t0))

	// TEARDOWN

	close(pipe2.StdIn)
	close(pipe2.StdOut)
	close(pipe2.StdErr)

	time.Sleep(100 * time.Millisecond)
}

// TestSequentialRun: same as TestCompiler, but with cache volumes and several compiler runs.
// - Go source references to external library
// - create Golang container with volumes
// - run the compilation N times
// - stop the Golang container
// - create runner container
// - run the binary in runner, stream output to caller
// - stop runner container
func TestSequentialRun(t *testing.T) {
	a := assert.New(t)

	logger, err := log.New("debug", false)
	a.NoError(err)

	cm, err := New(logger)
	a.NoError(err)

	setup := ContainerSetup{
		Image:      "golang:dummy",
		WorkingDir: workingDir,
		Label:      "compiler-golang-0.0",
		Envs:       map[string]string{"CGO_ENABLED": "0"},
		Resources: Resources{
			RAM:    256,
			CPUs:   1000,
			Net:    true,
			TmpDir: 40, // compiler needs some disk space to run
		},
		CacheVolume:      []string{"golang-go-volume", "golang-cache-volume"},
		CacheVolumeMount: []string{"/go/pkg", "/home/dummy/.cache/go-build"},
	}

	t0 := time.Now()

	ID, err := cm.CreateAndRunContainer(&setup)
	a.NoError(err)

	t.Logf("Compiler created in %v\n", time.Since(t0))

	// write cargo
	_ = os.WriteFile(filepath.Join(playgroundRoot, "home", "main.go"), []byte(strTestSequentialRun), 0755)

	pipe := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	limits := RuntimeLimits{
		CPUTime: 10000,
		Net:     10,
		RunTime: 50,
	}

	go streamReader(t, pipe)

	for n := 0; n < 4; n++ { // re-compile 4 times
		file.Delete(filepath.Join(playgroundRoot, "home", "go.mod"))
		file.Delete(filepath.Join(playgroundRoot, "home", "go.sum"))
		file.Delete(filepath.Join(playgroundRoot, "home", "main"))

		err = cm.WaitForIdle(ID, time.Second)
		a.NoError(err)

		t0 = time.Now()
		commands := []string{"sh", "-c", `echo "===== 1" >&2 ; go mod init dummy/module ; echo "===== 2" >&2 ; go mod tidy ; echo "===== 3" >&2 ; go build -trimpath -o main main.go`}
		code, err := cm.Execute(ID, commands, pipe, limits)
		t.Logf("Compiler executed (%v) in %v\n", n+1, time.Since(t0))
		a.NoError(err)
		a.Equal(0, code)
	}

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Compiler stopped in %v\n", time.Since(t0))

	close(pipe.StdIn)
	close(pipe.StdOut)
	close(pipe.StdErr)

	// RUN

	setup = ContainerSetup{
		Image:      "alpine:latest",
		WorkingDir: workingDir,
		Label:      "runner-golang-0.0",
		Resources: Resources{
			RAM:  256,
			CPUs: 1000,
			Net:  true,
		},
	}

	t0 = time.Now()

	ID, err = cm.CreateAndRunContainer(&setup)
	a.NoError(err)

	t.Logf("Runner created in %v\n", time.Since(t0))

	pipe2 := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	go streamReader(t, pipe2)

	t0 = time.Now()
	commands := []string{"./main"}
	code, err := cm.Execute(ID, commands, pipe2, limits)
	t.Logf("Runner executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Runner stopped in %v\n", time.Since(t0))

	// TEARDOWN

	close(pipe2.StdIn)
	close(pipe2.StdOut)
	close(pipe2.StdErr)

	time.Sleep(100 * time.Millisecond)
}

// TestSpawn: run the binary which spawns child process(es).
// - create a compiler container
// - compile the program
// - stop the compiler
// - create a runner container
// - run the binary spawning the new process
// - TODO: wait 1s
// - TODO: check that spawned process is removed
// - stop the runnerr container
func TestSpawn(t *testing.T) {
	a := assert.New(t)

	logger, err := log.New("debug", false)
	a.NoError(err)

	cm, err := New(logger)
	a.NoError(err)

	setup := ContainerSetup{
		Image:        "golang:dummy",
		WorkingDir:   workingDir,
		WorkingDirRO: false,
		Label:        "compiler-golang-0.0",
		Envs:         map[string]string{"CGO_ENABLED": "0"},
		Resources: Resources{
			RAM:    256,
			CPUs:   1000,
			Net:    true,
			TmpDir: 40,
		},
		CacheVolume:      []string{"golang-go-volume", "golang-cache-volume"},
		CacheVolumeMount: []string{"/go/pkg", "/home/dummy/.cache/go-build"},
	}

	t0 := time.Now()

	ID, err := cm.CreateAndRunContainer(&setup)
	a.NoError(err)

	t.Logf("Compiler created in %v\n", time.Since(t0))

	// write cargo
	_ = os.WriteFile(filepath.Join(playgroundRoot, "home", "main.go"), []byte(strTestSpawn), 0755)

	pipe := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	limits := RuntimeLimits{
		CPUTime: 10000,
		Net:     10,
		RunTime: 50,
	}

	go streamReader(t, pipe)

	file.Delete(filepath.Join(playgroundRoot, "home", "go.mod"))
	file.Delete(filepath.Join(playgroundRoot, "home", "go.sum"))
	file.Delete(filepath.Join(playgroundRoot, "home", "main"))

	t0 = time.Now()
	commands := []string{"sh", "-c", `go mod init dummy/module &> /dev/null ; go mod tidy ; go build -trimpath -o main main.go`}
	code, err := cm.Execute(ID, commands, pipe, limits)
	t.Logf("Compiler executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Compiler stopped in %v\n", time.Since(t0))

	close(pipe.StdIn)
	close(pipe.StdOut)
	close(pipe.StdErr)

	// RUN

	setup = ContainerSetup{
		Image:        "alpine:latest",
		WorkingDir:   workingDir,
		WorkingDirRO: true,
		Label:        "runner-golang-0.0",
		Resources: Resources{
			RAM:    256,  // Mb
			CPUs:   1000, // mCPUs
			Net:    true, // y/n
			TmpDir: 2,    // Mb
		},
	}

	t0 = time.Now()

	ID, err = cm.CreateAndRunContainer(&setup)
	a.NoError(err)
	if err != nil {
		t.Logf("err: %s\n", err.Error())
	}

	t.Logf("Runner created in %v\n", time.Since(t0))

	pipe2 := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}
	go streamReader(t, pipe2)

	t0 = time.Now()
	commands = []string{"./main"} // spawns the "sleep" process
	code, err = cm.Execute(ID, commands, pipe2, limits)
	t.Logf("Runner executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	// The main has finished, but the "sleep" process is still running.
	// The spawned processes are being terminated; we need to wait for it to finish
	err = cm.WaitForIdle(ID, time.Second)
	a.NoError(err)

	t0 = time.Now()
	commands = []string{"ps", "aux"}
	code, err = cm.Execute(ID, commands, pipe2, limits)
	t.Logf("Runner executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	t.Log("===> Should not see any \"sleep\" process in output below")

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Runner stopped in %v\n", time.Since(t0))

	// TEARDOWN

	close(pipe2.StdIn)
	close(pipe2.StdOut)
	close(pipe2.StdErr)

	time.Sleep(100 * time.Millisecond)
}

// TestRW: test tmp dir volume and RO on home dir
// TODO:
func TestRW(t *testing.T) {
	a := assert.New(t)

	logger, err := log.New("debug", false)
	a.NoError(err)

	cm, err := New(logger)
	a.NoError(err)

	setup := ContainerSetup{
		Image:        "golang:dummy",
		WorkingDir:   workingDir,
		WorkingDirRO: false,
		Label:        "compiler-golang-0.0",
		Envs:         map[string]string{"CGO_ENABLED": "0"},
		Resources: Resources{
			RAM:    256,
			CPUs:   1000,
			Net:    true,
			TmpDir: 20,
		},
		CacheVolume:      []string{"golang-go-volume", "golang-cache-volume"},
		CacheVolumeMount: []string{"/go/pkg", "/home/dummy/.cache/go-build"},
	}

	t0 := time.Now()

	ID, err := cm.CreateAndRunContainer(&setup)
	a.NoError(err)

	t.Logf("Compiler created in %v\n", time.Since(t0))

	// write cargo
	_ = os.WriteFile(filepath.Join(playgroundRoot, "home", "main.go"), []byte(strTestRW), 0755)

	pipe := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	limits := RuntimeLimits{
		CPUTime: 10000,
		Net:     10,
		RunTime: 50,
	}

	go streamReader(t, pipe)

	file.Delete(filepath.Join(playgroundRoot, "home", "go.mod"))
	file.Delete(filepath.Join(playgroundRoot, "home", "go.sum"))
	file.Delete(filepath.Join(playgroundRoot, "home", "main"))

	t0 = time.Now()
	commands := []string{"sh", "-c", `go mod init dummy/module &> /dev/null ; go mod tidy ; go build -trimpath -o main main.go`}
	code, err := cm.Execute(ID, commands, pipe, limits)
	t.Logf("Compiler executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Compiler stopped in %v\n", time.Since(t0))

	close(pipe.StdIn)
	close(pipe.StdOut)
	close(pipe.StdErr)

	// RUN

	setup = ContainerSetup{
		Image:        "alpine:latest",
		WorkingDir:   workingDir,
		WorkingDirRO: true,
		Label:        "runner-golang-0.0",
		Resources: Resources{
			RAM:    256,  // Mb
			CPUs:   1000, // mCPUs
			Net:    true, // y/n
			TmpDir: 2,    // Mb
		},
	}

	t0 = time.Now()

	ID, err = cm.CreateAndRunContainer(&setup)
	a.NoError(err)
	if err != nil {
		t.Logf("err: %s\n", err.Error())
	}

	t.Logf("Runner created in %v\n", time.Since(t0))

	pipe2 := ContainerPipe{
		StdIn:    make(chan []byte),
		StdOut:   make(chan []byte),
		StdErr:   make(chan []byte),
		Consumed: make(chan ConsumedResources),
	}

	go streamReader(t, pipe2)

	t0 = time.Now()
	commands = []string{"./main"}
	code, err = cm.Execute(ID, commands, pipe2, limits)
	t.Logf("Runner executed in %v\n", time.Since(t0))
	a.NoError(err)
	a.Equal(0, code)

	t0 = time.Now()
	cm.StopContainer(ID, false)
	t.Logf("Runner stopped in %v\n", time.Since(t0))

	// TEARDOWN

	close(pipe2.StdIn)
	close(pipe2.StdOut)
	close(pipe2.StdErr)

	time.Sleep(100 * time.Millisecond)
}
