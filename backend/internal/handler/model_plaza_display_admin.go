package handler

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminGetDisplay 返回广场展示配置。无效存档仍返回内置参考价，避免管理端无法修复。
func (h *ModelPlazaHandler) AdminGetDisplay(c *gin.Context) {
	if h == nil || h.settingService == nil {
		response.Error(c, http.StatusInternalServerError, "settings unavailable")
		return
	}
	cfg, err := h.settingService.GetModelPlazaDisplay(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrModelPlazaDisplayInvalid) {
			response.Success(c, service.ModelPlazaDisplayView{
				VideoReferenceSource: "invalid",
				InvalidReason:        err.Error(),
				VideoReferences:      service.DefaultPlazaVideoReferenceCatalog(),
				InsertAfter:          []service.PlazaGroupInsertAfter{},
			})
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg.View())
}

// AdminUpdateDisplay 保存广场展示配置。只写展示设置，不修改分组倍率或渠道价格。
func (h *ModelPlazaHandler) AdminUpdateDisplay(c *gin.Context) {
	if h == nil || h.settingService == nil {
		response.Error(c, http.StatusInternalServerError, "settings unavailable")
		return
	}
	var req service.ModelPlazaDisplayUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid model plaza display payload")
		return
	}
	view, err := h.settingService.UpdateModelPlazaDisplay(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}
