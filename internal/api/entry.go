package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
	"github.com/lynnyq/goldapadmin/internal/service"
)

// GetChildren 获取子节点
func (r *Router) GetChildren(c *gin.Context) {
	baseDN := c.Query("base_dn")
	if baseDN == "" {
		response.Fail(c, 400, "base_dn 不能为空")
		return
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
	start := time.Now()
	children, err := svc.GetChildren(conn, baseDN)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "list", baseDN, "", "failed", err.Error(), duration)
		response.Fail(c, 500, err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "list", baseDN, "", "success", "", duration)
	response.Success(c, children)
}

// GetTree 递归获取指定 baseDN 下所有子孙条目（嵌套树结构）
// GET /api/entry/tree?base_dn=xxx
func (r *Router) GetTree(c *gin.Context) {
	baseDN := c.Query("base_dn")
	if baseDN == "" {
		response.Fail(c, 400, "base_dn 不能为空")
		return
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
	start := time.Now()
	tree, err := svc.GetTree(conn, baseDN)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "tree", baseDN, "", "failed", err.Error(), duration)
		response.Fail(c, 500, err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "tree", baseDN, "", "success", "", duration)
	response.Success(c, tree)
}

// NextID 获取下一个可用的 uidNumber/gidNumber（对标 phpLDAPadmin 自增 ID 分配）
// GET /api/entry/next_id?type=uid|gid
func (r *Router) NextID(c *gin.Context) {
	kind := c.Query("type")
	if kind != "uid" && kind != "gid" {
		response.Fail(c, 400, "type 参数必须为 uid 或 gid")
		return
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
	next, err := svc.GetNextID(conn, params.Server.BaseDN, kind)
	if err != nil && !errors.Is(err, pkgldap.ErrSizeLimitTruncated) {
		response.Fail(c, 500, "查询自增 ID 失败: "+err.Error())
		return
	}
	// 被截断时返回保守估计值（部分数据最大+1），附带警告让调用方知晓
	// 若后续创建时真的重复，服务器会返回 EntryAlreadyExists，前端有友好提示
	if errors.Is(err, pkgldap.ErrSizeLimitTruncated) {
		c.JSON(200, response.Response{
			Code:    0,
			Message: "ok（因服务器 sizeLimit 截断，ID 为保守估计；若创建冲突请重新刷新）",
			Data:    gin.H{"next": next, "truncated": true},
		})
		return
	}
	response.Success(c, gin.H{"next": next})
}

// GetEntry 获取条目详情
func (r *Router) GetEntry(c *gin.Context) {
	dn := c.Query("dn")
	if dn == "" {
		response.Fail(c, 400, "dn 不能为空")
		return
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
	entry, err := svc.GetEntry(conn, dn)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, entry)
}

// CreateEntry 创建条目
func (r *Router) CreateEntry(c *gin.Context) {
	var req struct {
		DN         string              `json:"dn" binding:"required"`
		Attributes map[string][]string `json:"attributes" binding:"required"`
		// PasswordAlgos 密码属性哈希算法（对标 phpLDAPadmin password helper）
		// key=属性名，value=SSHA/SHA/MD5，空或缺省为明文
		PasswordAlgos map[string]string   `json:"password_algos"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
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
	start := time.Now()
	err = svc.CreateEntry(conn, req.DN, req.Attributes, req.PasswordAlgos)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "create", req.DN, "", "failed", err.Error(), duration)
		response.Fail(c, 500, "创建失败: "+err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "create", req.DN, "", "success", "", duration)
	response.Success(c, nil)
}

// ModifyEntry 修改条目
func (r *Router) ModifyEntry(c *gin.Context) {
	var req model.ModifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
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
	start := time.Now()
	err = svc.ModifyEntry(conn, req.DN, req.Changes, req.PasswordAlgos)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "modify", req.DN, "", "failed", err.Error(), duration)
		response.Fail(c, 500, "修改失败: "+err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "modify", req.DN, "", "success", "", duration)
	response.Success(c, nil)
}

// DeleteEntry 删除条目
func (r *Router) DeleteEntry(c *gin.Context) {
	dn := c.Query("dn")
	recursive := c.Query("recursive") == "true"
	if dn == "" {
		response.Fail(c, 400, "dn 不能为空")
		return
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
	start := time.Now()
	err = svc.DeleteEntry(conn, dn, recursive)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "delete", dn, "", "failed", err.Error(), duration)
		response.Fail(c, 500, "删除失败: "+err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "delete", dn, "", "success", "", duration)
	response.Success(c, nil)
}

// CopyEntry 复制条目（支持跨服务器 — 对标 phpLDAPadmin copy_form.php 的 server 下拉）
// 当 target_server_id 有值时，从当前 server 读取条目，连接目标 server 写入
func (r *Router) CopyEntry(c *gin.Context) {
	var req struct {
		SrcDN         string `json:"src_dn" binding:"required"`
		DestDN        string `json:"dest_dn" binding:"required"`
		Recursive     bool   `json:"recursive"`
		TargetServerID *int  `json:"target_server_id,omitempty"` // 跨服务器复制目标 server
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
	}

	params := r.mustConnParams(c)
	if params == nil {
		return
	}
	srcConn, err := r.svc.Connect(params)
	if err != nil {
		response.Fail(c, 500, "连接 LDAP 失败: "+err.Error())
		return
	}
	defer srcConn.Close()

	// 跨服务器场景：连接目标 server
	var destConn *ldap.Conn
	if req.TargetServerID != nil && *req.TargetServerID > 0 && *req.TargetServerID != params.Server.ID {
		targetSrv, err := r.svc.GetServerDAO().Get(*req.TargetServerID)
		if err != nil {
			response.Fail(c, 400, "目标服务器不存在: "+err.Error())
			return
		}
		// 用当前登录会话的凭据连接目标 server（同组织 LDAP 通常共享 bind 账号）
		destConn, err = r.svc.Connect(&service.ConnParams{
			Server:   targetSrv,
			BindDN:   params.BindDN,
			Password: params.Password,
		})
		if err != nil {
			response.Fail(c, 500, fmt.Sprintf("连接目标服务器失败（%s），请确认目标服务器接受当前凭据: %v", targetSrv.Name, err))
			return
		}
		defer destConn.Close()
	}

	svc := pkgldap.NewEntryService()
	start := time.Now()
	var copyErr error
	if destConn != nil {
		// 跨服务器：手动实现 CopyEntry 逻辑（读源连接、写目标连接）
		copyErr = svc.CopyEntryCrossServer(srcConn, destConn, req.SrcDN, req.DestDN, req.Recursive)
	} else {
		copyErr = svc.CopyEntry(srcConn, req.SrcDN, req.DestDN, req.Recursive)
	}
	duration := time.Since(start)
	if copyErr != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "copy", req.SrcDN, "", "failed", copyErr.Error(), duration)
		response.Fail(c, 500, "复制失败: "+copyErr.Error())
		return
	}

	targetLabel := ""
	if destConn != nil {
		targetLabel = fmt.Sprintf(" → target_server=%d", *req.TargetServerID)
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "copy", req.SrcDN, targetLabel, "success", "", duration)
	response.Success(c, nil)
}

// MoveEntry 移动条目
func (r *Router) MoveEntry(c *gin.Context) {
	var req struct {
		SrcDN      string `json:"src_dn" binding:"required"`
		NewParentDN string `json:"new_parent_dn" binding:"required"`
		NewRDN     string `json:"new_rdn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
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
	start := time.Now()
	err = svc.MoveEntry(conn, req.SrcDN, req.NewParentDN, req.NewRDN)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "move", req.SrcDN, "", "failed", err.Error(), duration)
		response.Fail(c, 500, "移动失败: "+err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "move", req.SrcDN, "", "success", "", duration)
	response.Success(c, nil)
}

// RenameEntry 重命名条目
func (r *Router) RenameEntry(c *gin.Context) {
	var req struct {
		DN    string `json:"dn" binding:"required"`
		NewRDN string `json:"new_rdn" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
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
	start := time.Now()
	err = svc.RenameEntry(conn, req.DN, req.NewRDN)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "rename", req.DN, "", "failed", err.Error(), duration)
		response.Fail(c, 500, "重命名失败: "+err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "rename", req.DN, "", "success", "", duration)
	response.Success(c, nil)
}

var _ = middleware.GetUserDN

// CompareEntry 执行 LDAP Compare 操作（对标 phpLDAPadmin compare.php）
// POST /api/entry/compare
func (r *Router) CompareEntry(c *gin.Context) {
	var req struct {
		DN    string `json:"dn" binding:"required"`
		Attr  string `json:"attr" binding:"required"`
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
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
	start := time.Now()
	matched, err := svc.CompareEntry(conn, req.DN, req.Attr, req.Value)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "compare", req.DN, req.Attr, "failed", err.Error(), duration)
		response.Fail(c, 500, "Compare 失败: "+err.Error())
		return
	}
	r.svc.LogOperation(params.Server.ID, params.BindDN, "compare", req.DN, req.Attr, "success", "", duration)
	response.Success(c, gin.H{"matched": matched})
}

// GetBinaryAttr 获取二进制属性的 base64 编码值（前端预览 jpegPhoto 用）
// GET /api/entry/binary?dn=xxx&attr=jpegPhoto
func (r *Router) GetBinaryAttr(c *gin.Context) {
	dn := c.Query("dn")
	attr := c.Query("attr")
	if dn == "" || attr == "" {
		response.Fail(c, 400, "dn 和 attr 不能为空")
		return
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
	data, err := svc.GetBinaryAttribute(conn, dn, attr)
	if err != nil {
		response.Fail(c, 404, err.Error())
		return
	}
	response.Success(c, gin.H{
		"base64": base64.StdEncoding.EncodeToString(data),
		"size":   len(data),
	})
}

// DownloadBinaryAttr 下载二进制属性原始字节流
// GET /api/entry/binary-download?dn=xxx&attr=userCertificate
func (r *Router) DownloadBinaryAttr(c *gin.Context) {
	dn := c.Query("dn")
	attr := c.Query("attr")
	if dn == "" || attr == "" {
		response.Fail(c, 400, "dn 和 attr 不能为空")
		return
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
	data, err := svc.GetBinaryAttribute(conn, dn, attr)
	if err != nil {
		response.Fail(c, 404, err.Error())
		return
	}

	// 猜 MIME + Content-Disposition 文件名
	mime := "application/octet-stream"
	filename := attr
	switch attr {
	case "jpegPhoto", "thumbnailPhoto":
		mime = "image/jpeg"
		filename = "photo.jpg"
	case "userCertificate":
		mime = "application/x-x509-user-cert"
		filename = "userCertificate.cer"
	case "userPKCS12":
		mime = "application/x-pkcs12"
		filename = "userPKCS12.p12"
	case "audio":
		mime = "audio/basic"
		filename = "audio.au"
	}
	c.Header("Content-Type", mime)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(200, mime, data)
}

// ListDescendants 列出 baseDN 下所有子孙条目 DN（递归删除预览用）
// GET /api/entry/descendants?base_dn=xxx
func (r *Router) ListDescendants(c *gin.Context) {
	baseDN := c.Query("base_dn")
	if baseDN == "" {
		response.Fail(c, 400, "base_dn 不能为空")
		return
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
	dns, err := svc.ListDescendants(conn, baseDN)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, gin.H{"count": len(dns), "dns": dns})
}

// BatchDelete 批量删除
// DELETE /api/entry/batch
func (r *Router) BatchDelete(c *gin.Context) {
	var req struct {
		DN        []string `json:"dns" binding:"required"`
		Recursive bool     `json:"recursive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
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
	start := time.Now()
	results, err := svc.BatchDelete(conn, req.DN, req.Recursive)
	duration := time.Since(start)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}

	ok := 0
	for _, item := range results {
		if item.Success {
			ok++
			r.svc.LogOperation(params.Server.ID, params.BindDN, "delete", item.DN, "", "success", "", item.Duration)
		} else {
			r.svc.LogOperation(params.Server.ID, params.BindDN, "delete", item.DN, "", "failed", item.Error, item.Duration)
		}
	}
	response.Success(c, gin.H{
		"total":   len(results),
		"success": ok,
		"failed":  len(results) - ok,
		"items":   results,
		"duration": duration,
	})
}

// BatchModify 批量修改
// POST /api/entry/batch-modify
func (r *Router) BatchModify(c *gin.Context) {
	var req struct {
		DN     []string `json:"dns" binding:"required"`
		Op     string   `json:"op" binding:"required"` // add / replace / delete
		Attr   string   `json:"attr" binding:"required"`
		Values []string `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
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
	start := time.Now()
	results, err := svc.BatchModify(conn, req.DN, req.Op, req.Attr, req.Values)
	duration := time.Since(start)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}

	ok := 0
	for _, item := range results {
		if item.Success {
			ok++
			r.svc.LogOperation(params.Server.ID, params.BindDN, "batch_modify", item.DN, req.Attr, "success", "", item.Duration)
		} else {
			r.svc.LogOperation(params.Server.ID, params.BindDN, "batch_modify", item.DN, req.Attr, "failed", item.Error, item.Duration)
		}
	}
	response.Success(c, gin.H{
		"total":    len(results),
		"success":  ok,
		"failed":   len(results) - ok,
		"items":    results,
		"duration": duration,
	})
}
