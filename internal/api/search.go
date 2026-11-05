package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/model"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// Search 搜索条目
//
// 安全措施：
//  1. filter 必须通过 ValidateFilter（长度/嵌套/括号配对），防止 LDAP 注入与异常请求
//  2. scope 必须是合法值（base/one/sub）
//  3. 未指定 size_limit 或 size_limit=0 时强制使用 DefaultSearchSizeLimit（1000），
//     防止全量拉取压垮 LDAP 服务器
func (r *Router) Search(c *gin.Context) {
	baseDN := c.Query("base_dn")
	filter := c.Query("filter")
	scope := c.DefaultQuery("scope", "sub")
	if baseDN == "" || filter == "" {
		response.Fail(c, 400, "base_dn 和 filter 不能为空")
		return
	}

	// 安全校验 1：filter 语法 + 边界
	if err := pkgldap.ValidateFilter(filter); err != nil {
		response.Fail(c, 400, "filter 非法: "+err.Error())
		return
	}

	// 安全校验 2：baseDN 语法必须合法
	if err := validateDN(baseDN); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	// 安全校验 3：scope 必须合法
	if _, err := pkgldap.ValidateScope(scope); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	sizeLimit := pkgldap.DefaultSearchSizeLimit // 默认强制上限
	if sl := c.Query("size_limit"); sl != "" {
		parsed, err := strconv.Atoi(sl)
		if err == nil && parsed > 0 {
			sizeLimit = parsed
		}
	}

	// 安全校验 3：size_limit 硬上限，防止过大值
	if sizeLimit > 10000 {
		response.Fail(c, 400, "size_limit 过大（最多 10000）")
		return
	}

	attrs := c.QueryArray("attributes")
	var attributes []string
	if len(attrs) > 0 {
		// 安全校验 4：属性名合法，禁止 "*"/"+" 等通配符（避免绕过敏感属性脱敏的意图）
		if err := pkgldap.ValidateAttributeNames(attrs); err != nil {
			response.Fail(c, 400, "attributes 非法: "+err.Error())
			return
		}
		attributes = attrs
	}

	params := r.mustConnParams(c)
	if params == nil {
		return
	}
	conn, err := r.svc.Connect(params)
	if err != nil {
		response.Fail(c, 500, "连接 LDAP 失败")
		return
	}
	defer conn.Close()

	svc := pkgldap.NewEntryService()
	entries, err := svc.Search(conn.Conn, baseDN, filter, scope, attributes, sizeLimit)
	if entries == nil {
		entries = []model.LdapEntry{}
	}

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
