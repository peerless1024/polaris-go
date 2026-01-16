package composite

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/polarismesh/polaris-go/pkg/log"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// doHealthCheck 执行健康检查
func doHealthCheck(port int, config *model.LosslessHealthCheckConfig) (bool, error) {
	// 构建健康检查 URL
	protocol := strings.ToLower(config.HealthCheckProtocol)
	url := fmt.Sprintf("%s://localhost:%d%s", protocol, port, config.HealthCheckPath)

	log.GetBaseLogger().Debugf("[LosslessController] doHealthCheck, url: %s, method: %s",
		url, config.HealthCheckMethod)

	// 创建 HTTP 请求
	req, err := http.NewRequest(config.HealthCheckMethod, url, nil)
	if err != nil {
		log.GetBaseLogger().Errorf("[LosslessController] doHealthCheck, create request failed, err: %v", err)
		return false, err
	}

	// 设置超时时间（使用检查间隔的一半作为超时时间，但最少1秒）
	timeout := config.HealthCheckInterval / 2
	if timeout < time.Second {
		timeout = time.Second
	}
	client := &http.Client{
		Timeout: timeout,
	}

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		log.GetBaseLogger().Errorf("[LosslessController] doHealthCheck, request failed, err: %v", err)
		return false, err
	}
	defer resp.Body.Close()

	// 判断响应状态码，2xx 表示健康检查成功
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.GetBaseLogger().Infof("[LosslessController] doHealthCheck, health check success, statusCode: %d",
			resp.StatusCode)
		return true, nil
	}
	log.GetBaseLogger().Errorf("[LosslessController] doHealthCheck, health check failed, statusCode: %d, need retry",
		resp.StatusCode)
	return false, nil
}
