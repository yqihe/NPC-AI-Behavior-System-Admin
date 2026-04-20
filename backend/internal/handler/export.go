package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// ExportHandler 配置导出 API
//
// 统一放所有 /api/configs/* 导出接口。
// 不走 WrapCtx（导出 API 格式与 CRUD 不同）。
type ExportHandler struct {
	eventTypeService *service.EventTypeService
	fsmConfigService *service.FsmConfigService
	btTreeService    *service.BtTreeService
	npcService       *service.NpcService
	regionService    *service.RegionService
}

// NewExportHandler 创建 ExportHandler
func NewExportHandler(
	eventTypeService *service.EventTypeService,
	fsmConfigService *service.FsmConfigService,
	btTreeService *service.BtTreeService,
	npcService *service.NpcService,
	regionService *service.RegionService,
) *ExportHandler {
	return &ExportHandler{
		eventTypeService: eventTypeService,
		fsmConfigService: fsmConfigService,
		btTreeService:    btTreeService,
		npcService:       npcService,
		regionService:    regionService,
	}
}

// exportResponse 导出 API 统一响应格式
type exportResponse struct {
	Items interface{} `json:"items"`
}

// EventTypes GET /api/configs/event_types
//
// 返回所有已启用且未删除的事件类型。
// config 字段直接从 config_json 列原样展开，不经过 Go struct 中转。
func (h *ExportHandler) EventTypes(c *gin.Context) {
	slog.Debug("handler.export.event_types")

	items, err := h.eventTypeService.ExportAll(c.Request.Context())
	if err != nil {
		h.respondInternalErr(c, "event_types", "export", err)
		return
	}

	// 空数据返回 {"items": []}
	if len(items) == 0 {
		c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
		return
	}

	c.JSON(http.StatusOK, exportResponse{Items: items})
}

// FsmConfigs GET /api/configs/fsm_configs
//
// 返回所有已启用且未删除的状态机配置。
// config 字段直接从 config_json 列原样展开，不经过 Go struct 中转。
func (h *ExportHandler) FsmConfigs(c *gin.Context) {
	slog.Debug("handler.export.fsm_configs")

	items, err := h.fsmConfigService.ExportAll(c.Request.Context())
	if err != nil {
		h.respondInternalErr(c, "fsm_configs", "export", err)
		return
	}

	// 空数据返回 {"items": []}
	if len(items) == 0 {
		c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
		return
	}

	c.JSON(http.StatusOK, exportResponse{Items: items})
}

// BTTrees GET /api/configs/bt_trees
//
// 返回所有已启用且未删除的行为树。
// config 字段直接从 config 列原样展开，不经过 Go struct 中转。
func (h *ExportHandler) BTTrees(c *gin.Context) {
	slog.Debug("handler.export.bt_trees")

	items, err := h.btTreeService.ExportAll(c.Request.Context())
	if err != nil {
		h.respondInternalErr(c, "bt_trees", "export", err)
		return
	}

	// 空数据返回 {"items": []}
	if len(items) == 0 {
		c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
		return
	}

	c.JSON(http.StatusOK, exportResponse{Items: items})
}

// NPCTemplates GET /api/configs/npc_templates
//
// 返回所有已启用且未删除的 NPC 配置（含字段值快照 + 行为配置）。
// 格式：{name, config: {template_ref, fields: {k:v}, behavior: {fsm_ref?, bt_refs?}}}
//
// 5 步编排（NpcService 不持有 fsm/bt service，跨模块校验由 handler 编排，
// 详见 docs/specs/export-ref-validation/design.md §1.1）：
//  1. ExportRows 取原始 NPC 行
//  2. CollectExportRefs 构建反查索引
//  3a/3b. fsm/bt service 批量校验 enabled
//  4. BuildExportDanglingError 拼 details，悬空 → 5xx + code 45016
//  5. AssembleExportItems 装配 → 200
func (h *ExportHandler) NPCTemplates(c *gin.Context) {
	ctx := c.Request.Context()
	slog.Debug("handler.export.npc_templates")

	// Step 1: 取行
	rows, err := h.npcService.ExportRows(ctx)
	if err != nil {
		h.respondInternalErr(c, "npc_templates", "export_rows", err)
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
		return
	}

	// Step 2: 收集引用反查索引
	refs, err := h.npcService.CollectExportRefs(rows)
	if err != nil {
		h.respondInternalErr(c, "npc_templates", "collect_refs", err)
		return
	}

	// Step 3: 跨模块校验（key 集合空时 helper 自动短路不发 SQL）
	fsmNames := make([]string, 0, len(refs.FsmIndex))
	for name := range refs.FsmIndex {
		fsmNames = append(fsmNames, name)
	}
	fsmNotOK, err := h.fsmConfigService.CheckEnabledByNames(ctx, fsmNames)
	if err != nil {
		h.respondInternalErr(c, "npc_templates", "check_fsm", err)
		return
	}
	btNames := make([]string, 0, len(refs.BtIndex))
	for name := range refs.BtIndex {
		btNames = append(btNames, name)
	}
	btNotOK, err := h.btTreeService.CheckEnabledByNames(ctx, btNames)
	if err != nil {
		h.respondInternalErr(c, "npc_templates", "check_bt", err)
		return
	}

	// Step 4: 构建 dangling error
	if dangling := h.npcService.BuildExportDanglingError(refs, fsmNotOK, btNotOK); dangling != nil {
		slog.Error("handler.export.npc_templates.dangling_refs",
			"count", len(dangling.Details), "details", dangling.Details)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    errcode.ErrNPCExportDanglingRef,
			"message": errcode.Msg(errcode.ErrNPCExportDanglingRef),
			"details": dangling.Details,
		})
		return
	}

	// Step 5: 装配
	items, err := h.npcService.AssembleExportItems(rows)
	if err != nil {
		h.respondInternalErr(c, "npc_templates", "assemble", err)
		return
	}
	c.JSON(http.StatusOK, exportResponse{Items: items})
}

// Regions GET /api/configs/regions
//
// 返回所有已启用且未删除的区域配置（含 spawn_table + NPC template 引用）。
// 格式：{name: region_id, config: {region_id, name: display_name, region_type, spawn_table}}
//
// 5 步编排（RegionService 不持有 npcService 作导出用途，跨模块校验由 handler 编排，
// 对齐 NPCTemplates 的分层规则）：
//  1. ExportRows 取原始 region 行
//  2. CollectExportRefs 构建 template_ref → region_id 反查索引
//  3. npcService.LookupByNames 批量查 enabled 状态，分类 notOK（不存在 OR 未启用）
//  4. BuildExportDanglingError 拼 details，悬空 → 5xx + code 47011
//  5. AssembleExportItems 装配 → 200
func (h *ExportHandler) Regions(c *gin.Context) {
	ctx := c.Request.Context()
	slog.Debug("handler.export.regions")

	// Step 1: 取行
	rows, err := h.regionService.ExportRows(ctx)
	if err != nil {
		h.respondInternalErr(c, "regions", "export_rows", err)
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
		return
	}

	// Step 2: 收集引用反查索引
	refs, err := h.regionService.CollectExportRefs(rows)
	if err != nil {
		h.respondInternalErr(c, "regions", "collect_refs", err)
		return
	}

	// Step 3: 跨模块校验（template_ref 所指 NPC 模板 enabled 状态）
	templateNames := make([]string, 0, len(refs.TemplateIndex))
	for name := range refs.TemplateIndex {
		templateNames = append(templateNames, name)
	}
	statusMap, err := h.npcService.LookupByNames(ctx, templateNames)
	if err != nil {
		h.respondInternalErr(c, "regions", "check_npc_templates", err)
		return
	}
	templateNotOK := make([]string, 0)
	for _, name := range templateNames {
		if enabled, exists := statusMap[name]; !exists || !enabled {
			templateNotOK = append(templateNotOK, name)
		}
	}

	// Step 4: 构建 dangling error
	if dangling := h.regionService.BuildExportDanglingError(refs, templateNotOK); dangling != nil {
		slog.Error("handler.export.regions.dangling_refs",
			"count", len(dangling.Details), "details", dangling.Details)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    errcode.ErrRegionExportDanglingRef,
			"message": errcode.Msg(errcode.ErrRegionExportDanglingRef),
			"details": dangling.Details,
		})
		return
	}

	// Step 5: 装配
	items := h.regionService.AssembleExportItems(rows)
	c.JSON(http.StatusOK, exportResponse{Items: items})
}

// respondInternalErr 统一通用 500 响应（含中文 message + slog 原始 error）
//
// 符合 admin red-line #14.2：所有 4xx/5xx 响应必须含 code 字段。
// endpoint：导出端点名（event_types / fsm_configs / bt_trees / npc_templates），用于 slog 分流。
// stage：NPCTemplates 多步编排的阶段标记；简单端点固定 "export"。
func (h *ExportHandler) respondInternalErr(c *gin.Context, endpoint, stage string, err error) {
	slog.Error("handler.export."+endpoint+".error", "stage", stage, "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    errcode.ErrInternal,
		"message": "导出失败，请查看服务端日志",
	})
}
