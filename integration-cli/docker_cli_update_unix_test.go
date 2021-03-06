// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestUpdateRunningContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "top")
	dockerCmd(c, "update", "-m", "500M", name)

	memory := inspectField(c, name, "HostConfig.Memory")
	if memory != "524288000" {
		c.Fatalf("Got the wrong memory value, we got %d, expected 524288000(500M).", memory)
	}

	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "524288000")
}

func (s *DockerSuite) TestUpdateRunningContainerWithRestart(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "top")
	dockerCmd(c, "update", "-m", "500M", name)
	dockerCmd(c, "restart", name)

	memory := inspectField(c, name, "HostConfig.Memory")
	if memory != "524288000" {
		c.Fatalf("Got the wrong memory value, we got %d, expected 524288000(500M).", memory)
	}

	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "524288000")
}

func (s *DockerSuite) TestUpdateStoppedContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	dockerCmd(c, "run", "--name", name, "-m", "300M", "busybox", "cat", file)
	dockerCmd(c, "update", "-m", "500M", name)

	memory := inspectField(c, name, "HostConfig.Memory")
	if memory != "524288000" {
		c.Fatalf("Got the wrong memory value, we got %d, expected 524288000(500M).", memory)
	}

	out, _ := dockerCmd(c, "start", "-a", name)
	c.Assert(strings.TrimSpace(out), checker.Equals, "524288000")
}

func (s *DockerSuite) TestUpdatePausedContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, cpuShare)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "--cpu-shares", "1000", "busybox", "top")
	dockerCmd(c, "pause", name)
	dockerCmd(c, "update", "--cpu-shares", "500", name)

	out := inspectField(c, name, "HostConfig.CPUShares")
	if out != "500" {
		c.Fatalf("Got the wrong cpu shares value, we got %d, expected 500.", out)
	}

	dockerCmd(c, "unpause", name)
	file := "/sys/fs/cgroup/cpu/cpu.shares"
	out, _ = dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "500")
}

func (s *DockerSuite) TestUpdateWithUntouchedFields(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, cpuShare)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "--cpu-shares", "800", "busybox", "top")
	dockerCmd(c, "update", "-m", "500M", name)

	// Update memory and not touch cpus, `cpuset.cpus` should still have the old value
	out := inspectField(c, name, "HostConfig.CPUShares")
	c.Assert(out, check.Equals, "800")

	file := "/sys/fs/cgroup/cpu/cpu.shares"
	out, _ = dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "800")
}

func (s *DockerSuite) TestUpdateContainerInvalidValue(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "true")
	out, _, err := dockerCmdWithError("update", "-m", "2M", name)
	c.Assert(err, check.NotNil)
	expected := "Minimum memory limit allowed is 4MB"
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestUpdateContainerWithoutFlags(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "true")
	_, _, err := dockerCmdWithError("update", name)
	c.Assert(err, check.NotNil)
}

func (s *DockerSuite) TestUpdateKernelMemory(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, kernelMemorySupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "--kernel-memory", "50M", "busybox", "top")
	_, _, err := dockerCmdWithError("update", "--kernel-memory", "100M", name)
	// Update kernel memory to a running container is not allowed.
	c.Assert(err, check.NotNil)

	out := inspectField(c, name, "HostConfig.KernelMemory")
	// Update kernel memory to a running container with failure should not change HostConfig
	if out != "52428800" {
		c.Fatalf("Got the wrong memory value, we got %d, expected 52428800(50M).", out)
	}

	dockerCmd(c, "stop", name)
	dockerCmd(c, "update", "--kernel-memory", "100M", name)
	dockerCmd(c, "start", name)

	out = inspectField(c, name, "HostConfig.KernelMemory")
	if out != "104857600" {
		c.Fatalf("Got the wrong memory value, we got %d, expected 104857600(100M).", out)
	}

	file := "/sys/fs/cgroup/memory/memory.kmem.limit_in_bytes"
	out, _ = dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "104857600")
}

func (s *DockerSuite) TestUpdateStats(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, cpuCfsQuota)
	name := "foo"
	dockerCmd(c, "run", "-d", "-ti", "--name", name, "-m", "500m", "busybox")

	c.Assert(waitRun(name), checker.IsNil)

	getMemLimit := func(id string) uint64 {
		resp, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats?stream=false", id), nil, "")
		c.Assert(err, checker.IsNil)
		c.Assert(resp.Header.Get("Content-Type"), checker.Equals, "application/json")

		var v *types.Stats
		err = json.NewDecoder(body).Decode(&v)
		c.Assert(err, checker.IsNil)
		body.Close()

		return v.MemoryStats.Limit
	}
	preMemLimit := getMemLimit(name)

	dockerCmd(c, "update", "--cpu-quota", "2000", name)

	curMemLimit := getMemLimit(name)

	c.Assert(preMemLimit, checker.Equals, curMemLimit)

}
