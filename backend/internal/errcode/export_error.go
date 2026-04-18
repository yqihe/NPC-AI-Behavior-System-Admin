package errcode

import (
	"fmt"

	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ExportDanglingRefError NPC 导出期发现悬空引用的结构化错误
//
// service.NpcService.BuildExportDanglingError 构造并返回；
// handler 直接用返回值 nil 检查（编排式），渲染为 5xx + code=ErrNPCExportDanglingRef +
// details 数组的 JSON。Details 携带全部悬空条目（不止第一个）。
//
// 实现 error 接口（Go 惯例 + 未来 errors.As 包装链路扩展点保留）。
type ExportDanglingRefError struct {
	Details []model.NPCExportDanglingRef
}

func (e *ExportDanglingRefError) Error() string {
	return fmt.Sprintf("npc export found %d dangling refs", len(e.Details))
}
