import request from './request'
import type { ApiResponse } from './request'
import type { ListData, CheckNameResult, DeleteResult } from './fields'

// ============================================================================
// 类型定义（与 backend/internal/model/npc.go 的 JSON tag 严格对齐）
// ============================================================================

/** 列表查询参数 */
export interface NPCListQuery {
  name?: string
  label?: string
  /** 精确匹配模板标识 */
  template_name?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

/** 列表项（handler 层跨模块补全 template_label） */
export interface NPCListItem {
  id: number
  name: string
  label: string
  template_name: string
  template_label: string
  fsm_ref: string
  enabled: boolean
  created_at: string
}

/** 详情中的字段值项 */
export interface NPCDetailField {
  field_id: number
  name: string
  label: string
  type: string
  category: string
  category_label: string
  enabled: boolean
  required: boolean
  value: unknown
}

/** 详情（handler 层拼装，含字段值快照 + 行为配置） */
export interface NPCDetail {
  id: number
  name: string
  label: string
  description: string
  template_id: number
  template_name: string
  template_label: string
  enabled: boolean
  version: number
  fields: NPCDetailField[]
  fsm_ref: string
  /** state_name → bt_tree_name */
  bt_refs: Record<string, string>
}

/** 单个字段值（提交用） */
export interface NPCFieldValue {
  field_id: number
  value: number | string | boolean | null
}

/** 创建请求 */
export interface CreateNPCRequest {
  name: string
  label: string
  description?: string
  template_id: number
  field_values: NPCFieldValue[]
  fsm_ref?: string
  /** state_name → bt_tree_name */
  bt_refs?: Record<string, string>
}

/** 编辑请求（无 name + template_id，均不可变） */
export interface UpdateNPCRequest {
  id: number
  label: string
  description?: string
  field_values: NPCFieldValue[]
  fsm_ref?: string
  bt_refs?: Record<string, string>
  version: number
}

/** 创建响应 */
export interface CreateNPCResponse {
  id: number
  name: string
}

// Re-export 共享类型（避免消费方多处 import）
export type { ListData, CheckNameResult, DeleteResult }

// ============================================================================
// 错误码常量（45001–45015，与 backend/internal/errcode/codes.go 保持一致）
// ============================================================================

export const NPC_ERRORS = {
  NAME_EXISTS:          45001,
  NAME_INVALID:         45002,
  NOT_FOUND:            45003,
  TEMPLATE_NOT_FOUND:   45004,
  TEMPLATE_DISABLED:    45005,
  FIELD_VALUE_INVALID:  45006,
  FIELD_REQUIRED:       45007,
  FSM_NOT_FOUND:        45008,
  FSM_DISABLED:         45009,
  BT_NOT_FOUND:         45010,
  BT_DISABLED:          45011,
  BT_STATE_INVALID:     45012,
  DELETE_NOT_DISABLED:  45013,
  VERSION_CONFLICT:     45014,
  BT_WITHOUT_FSM:       45015,
} as const

// ============================================================================
// API 对象
// ============================================================================

export const npcApi = {
  list: (params: NPCListQuery) =>
    request.post('/npcs/list', params) as Promise<ApiResponse<ListData<NPCListItem>>>,

  create: (data: CreateNPCRequest) =>
    request.post('/npcs/create', data) as Promise<ApiResponse<CreateNPCResponse>>,

  detail: (id: number) =>
    request.post('/npcs/detail', { id }) as Promise<ApiResponse<NPCDetail>>,

  update: (data: UpdateNPCRequest) =>
    request.post('/npcs/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/npcs/delete', { id }) as Promise<ApiResponse<DeleteResult>>,

  checkName: (name: string) =>
    request.post('/npcs/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/npcs/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
