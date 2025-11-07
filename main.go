package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Request 定义客户端请求格式，binding:"required" 确保字段必填
type Request struct {
	Cmd  string   `json:"cmd" binding:"required"`
	Args []string `json:"args"`
}

// whitelist 存储允许的命令列表，whitelistEnabled 指示是否启用白名单
var (
	whitelist        map[string]bool
	whitelistEnabled bool
)

func init() {
	// 初始化日志
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	// 默认级别为 Info，可以通过环境变量或配置调整为 Debug
	logrus.SetLevel(logrus.DebugLevel)

	// 从环境变量读取白名单，使用逗号分隔
	if env := os.Getenv("WHITELIST_CMDS"); env != "" {
		whitelistEnabled = true
		whitelist = make(map[string]bool)
		for _, cmd := range strings.Split(env, ",") {
			cmd = strings.TrimSpace(cmd)
			if cmd != "" {
				whitelist[cmd] = true
				logrus.Debugf("Whitelist enabled, allow command: %s", cmd)
			}
		}
		logrus.Infof("Whitelist enabled with %d commands", len(whitelist))
	} else {
		logrus.Info("Whitelist disabled")
	}
}

func main() {
	logrus.Info("Starting server on :8080")
	r := gin.Default()
	r.POST("/execute", executeHandler)
	if err := r.Run(); err != nil {
		logrus.Fatalf("Failed to run server: %v", err)
	}
}

// executeHandler 流式执行命令，若启用白名单则校验
func executeHandler(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("Bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logrus.Infof("Received request: cmd=%s args=%v", req.Cmd, req.Args)

	// 如果启用了白名单，则验证命令
	if whitelistEnabled {
		logrus.Debugf("Checking whitelist for command: %s", req.Cmd)
		if !whitelist[req.Cmd] {
			msg := fmt.Sprintf("command '%s' not allowed", req.Cmd)
			logrus.Warn(msg)
			c.JSON(http.StatusForbidden, gin.H{"error": msg})
			return
		}
		logrus.Debugf("Command '%s' is allowed", req.Cmd)
	}

	// 使用带超时的上下文，防止命令无限挂起（可根据需求调整）
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	logrus.Debug("Created context with 5m timeout")

	cmd := exec.CommandContext(ctx, req.Cmd, req.Args...)
	logrus.Debugf("Prepared exec.Command: %s %v", req.Cmd, req.Args)

	// 获取输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logrus.Errorf("StdoutPipe error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logrus.Errorf("StderrPipe error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		logrus.Errorf("Command start failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logrus.Infof("Started command PID=%d", cmd.Process.Pid)

	// 流式输出
	reader := bufio.NewReader(io.MultiReader(stdout, stderr))
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	c.Stream(func(w io.Writer) bool {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			// 这里也可以根据需要输出 Debug 级别
			logrus.Debugf("Cmd output: %s", strings.TrimRight(line, "\n"))
			w.Write([]byte(line))
		}
		return err == nil
	})

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		logrus.Errorf("Command exited with error: %v", err)
	} else {
		logrus.Infof("Command PID=%d completed successfully", cmd.Process.Pid)
	}
}
