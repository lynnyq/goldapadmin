package api

import (
	"time"

	"github.com/gin-gonic/gin"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// GetMonitorStats 获取监控统计
func (r *Router) GetMonitorStats(c *gin.Context) {
	serverID := middleware.GetServerID(c)
	params := r.mustConnParams(c)
	if params == nil {
		return
	}

	// 测试连接
	conn, connErr := r.svc.Connect(params)
	connected := connErr == nil
	if connected {
		conn.Close()
	}

	// 从日志获取统计
	total, success, failed, avgMs, _ := r.svc.GetLogDAO().StatsByServer(serverID)
	opsByType, _ := r.svc.GetLogDAO().CountByAction(serverID)

	// 获取最近响应时间
	times, _ := r.svc.GetLogDAO().RecentResponseTimes(serverID, 50)
	responseTimes := make([]gin.H, 0, len(times))
	for _, t := range times {
		responseTimes = append(responseTimes, gin.H{"time": t.Time, "avg_ms": t.AvgMS})
	}

	stats := gin.H{
		"connected":          connected,
		"active_connections": 1,
		"total_operations":   total,
		"success_count":      success,
		"failed_count":       failed,
		"avg_response_ms":    avgMs,
		"uptime_seconds":     int64(time.Since(startTime).Seconds()),
		"operations_by_type": opsByType,
		"response_times":     responseTimes,
	}

	if connErr != nil {
		stats["error"] = connErr.Error()
	}

	response.Success(c, stats)
}

// startTime 服务启动时间
var startTime = time.Now()

// ensure import used
var _ = pkgldap.NewEntryService
