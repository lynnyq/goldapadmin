package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// GetLogs 获取操作日志
func (r *Router) GetLogs(c *gin.Context) {
	// 操作日志包含同服务器其他用户的操作 DN，仅管理员可见
	if !r.svc.IsAdmin(middleware.GetUserDN(c)) {
		response.Fail(c, 403, "无权限：仅管理员可查看操作日志")
		return
	}
	serverID := middleware.GetServerID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	list, total, err := r.svc.GetLogDAO().List(serverID, page, pageSize)
	if err != nil {
		response.Fail(c, 500, "获取操作日志失败")
		return
	}
	if list == nil {
		list = []model.OperationLog{}
	}
	response.Success(c, gin.H{"total": total, "list": list})
}
