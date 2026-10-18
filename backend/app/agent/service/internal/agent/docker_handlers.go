// docker_handlers.go 实现 Docker 镜像部署指令(docker_deploy):
// pull 镜像 → 强制删除同名旧容器 → run -d 新容器。
// 目标机器需已安装 docker CLI;经 RunArgv 直连 argv 执行,不经 shell。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-wind-ops/pkg/shellexec"
	"go-wind-ops/pkg/signal"
)

const (
	dockerPullTimeout = 10 * time.Minute
	dockerRunTimeout  = 2 * time.Minute
)

// registerDockerHandlers 注册 Docker 部署指令处理器。
func (s *Service) registerDockerHandlers() {
	s.RegisterHandler(signal.CommandActionDockerDeploy, s.handleDockerDeploy)
}

// handleDockerDeploy 执行镜像部署并回执容器 ID。
func (s *Service) handleDockerDeploy(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := &signal.DockerDeployPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if payload.Image == "" || payload.ContainerName == "" {
		return nil, fmt.Errorf("payload.image/container_name are required")
	}

	// ── 1. 拉取镜像 ──
	code, out, timedOut, err := shellexec.RunArgv(ctx, "", dockerPullTimeout, "docker", "pull", payload.Image)
	if err != nil || timedOut || code != 0 {
		return nil, fmt.Errorf("docker pull %s: %s", payload.Image, summarizeDockerOutput(out, err, timedOut))
	}
	s.log().Infof("docker image pulled (dist=%d image=%s)", payload.DistributionId, payload.Image)

	// ── 2. 强制删除同名旧容器(不存在时报错忽略,保持幂等) ──
	_, _, _, _ = shellexec.RunArgv(ctx, "", dockerRunTimeout, "docker", "rm", "-f", "-v", payload.ContainerName)

	// ── 3. 启动新容器 ──
	restart := payload.Restart
	if restart == "" {
		restart = "unless-stopped"
	}
	args := []string{"run", "-d", "--name", payload.ContainerName, "--restart", restart}
	for _, p := range payload.Ports {
		args = append(args, "-p", p)
	}
	for _, e := range payload.Envs {
		args = append(args, "-e", e)
	}
	for _, v := range payload.Volumes {
		args = append(args, "-v", v)
	}
	if payload.Network != "" {
		args = append(args, "--network", payload.Network)
	}
	args = append(args, payload.Image)

	code, out, timedOut, err = shellexec.RunArgv(ctx, "", dockerRunTimeout, "docker", args...)
	if err != nil || timedOut || code != 0 {
		return nil, fmt.Errorf("docker run %s: %s", payload.ContainerName, summarizeDockerOutput(out, err, timedOut))
	}

	containerId := strings.TrimSpace(out)
	if len(containerId) > 12 {
		containerId = containerId[:12]
	}

	s.log().Infof("docker container deployed (dist=%d container=%s id=%s)",
		payload.DistributionId, payload.ContainerName, containerId)
	return &signal.DockerDeployResult{
		Image:         payload.Image,
		ContainerName: payload.ContainerName,
		ContainerId:   containerId,
	}, nil
}

// summarizeDockerOutput 归并 docker 命令失败信息(输出截断到 2KB)。
func summarizeDockerOutput(out string, err error, timedOut bool) string {
	switch {
	case timedOut:
		return "timeout"
	case err != nil:
		return err.Error()
	default:
		out = strings.TrimSpace(out)
		if len(out) > 2048 {
			out = out[:2048] + "...(truncated)"
		}
		if out == "" {
			return "non-zero exit"
		}
		return out
	}
}
