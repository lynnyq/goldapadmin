package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// ListBookmarks 获取书签列表
func (r *Router) ListBookmarks(c *gin.Context) {
	serverID := middleware.GetServerID(c)
	list, err := r.svc.GetBookmarkDAO().ListByServer(serverID)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	if list == nil {
		list = []model.Bookmark{}
	}
	response.Success(c, list)
}

// CreateBookmark 创建书签
func (r *Router) CreateBookmark(c *gin.Context) {
	var b model.Bookmark
	if err := c.ShouldBindJSON(&b); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}
	b.ServerID = middleware.GetServerID(c)
	if err := r.svc.GetBookmarkDAO().Create(&b); err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, b)
}

// DeleteBookmark 删除书签
func (r *Router) DeleteBookmark(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := r.svc.GetBookmarkDAO().Delete(id); err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, nil)
}
