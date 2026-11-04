package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
)

// Search 搜索条目
func (r *Router) Search(c *gin.Context) {
	baseDN := c.Query("base_dn")
	filter := c.Query("filter")
	scope := c.DefaultQuery("scope", "sub")
	if baseDN == "" || filter == "" {
		response.Fail(c, 400, "base_dn 和 filter 不能为空")
		return
	}

	sizeLimit := 0
	if sl := c.Query("size_limit"); sl != "" {
		sizeLimit, _ = strconv.Atoi(sl)
	}

	// 读取前端指定的属性列表（如 gidNumber 选择器只需要 cn、gidNumber）；
	// 未指定时传 nil，由 EntryService.Search 用默认属性集
	attrs := c.QueryArray("attributes")
	var attributes []string
	if len(attrs) > 0 {
		attributes = attrs
	}

	params := r.mustConnParams(c)
	if params == nil {
		return
	}
	conn, err := r.svc.Connect(params)
	if err != nil {
		response.Fail(c, 500, "连接 LDAP 失败: "+err.Error())
		return
	}
	defer conn.Close()

	svc := pkgldap.NewEntryService()
	entries, err := svc.Search(conn, baseDN, filter, scope, attributes, sizeLimit)
	if entries == nil {
		entries = []model.LdapEntry{}
	}

	// 被服务器 sizelimit 截断但返回了部分数据 → 以 code=0 + truncated 消息返回，
	// 前端可正常渲染部分结果并展示"结果被截断"警告（对标 phpLDAPadmin 行为）
	if errors.Is(err, pkgldap.ErrSizeLimitTruncated) {
		c.JSON(200, response.Response{
			Code:    0,
			Message: "ok（结果因服务器 sizeLimit 被截断，仅返回前 " + strconv.Itoa(len(entries)) + " 条）",
			Data:    entries,
		})
		return
	}
	if err != nil {
		response.Fail(c, 500, "搜索失败: "+err.Error())
		return
	}
	response.Success(c, entries)
}
