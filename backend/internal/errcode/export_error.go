package errcode

import (
	"fmt"

	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ExportDanglingRefError NPC 导出期发现悬空引用的结构化错误
//
// service.NpcService.ExportAll 在引用复核失败时返回；
// handler 用 errors.As 提取 Details，渲染为 5xx + code=ErrNPCExportDanglingRef +
// details 数组的 JSON。Details 携带全部悬空条目（不止第一个）。
type ExportDanglingRefError struct {
	Details []model.NPCExportDanglingRef
}

func (e *ExportDanglingRefError) Error() string {
	return fmt.Sprintf("npc export found %d dangling refs", len(e.Details))
}
