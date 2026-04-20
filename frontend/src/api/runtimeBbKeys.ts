import request from './request'
import type { ApiResponse } from './request'
import type { ListData, ReferenceItem, CheckNameResult, DeleteResult } from './fields'

/** 运行时 BB Key 列表查询参数（对齐 backend model.RuntimeBbKeyListQuery） */
export interface RuntimeBbKeyListQuery {
  name?: string
  label?: string
  type?: string          // integer / float / string / bool
  group_name?: string    // 11 枚举之一
  enabled?: boolean | null
  page: number
  page_size: number
}

/** 列表项（覆盖索引返回，不含 description/updated_at） */
export interface RuntimeBbKeyListItem {
  id: number
  name: string
  type: string
  label: string
  group_name: string
  enabled: boolean
  created_at: string
}

/** 详情（含 version / has_refs / ref_count） */
export interface RuntimeBbKey {
  id: number
  name: string
  type: string
  label: string
  description: string
  group_name: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
  has_refs: boolean
  ref_count: number
}

/** 引用详情（FSM/BT 分组） */
export interface RuntimeBbKeyReferenceDetail {
  key_id: number
  key_name: string
  key_label: string
  fsms: ReferenceItem[]
  bts: ReferenceItem[]
}

// 运行时 BB Key 段错误码（46001-46011，与 backend/internal/errcode/codes.go 保持一致）
// 40018 是 field 反向冲突码（字段 name 与 runtime key 冲突时由 /fields/* 返回）
export const RUNTIME_BB_KEY_ERR = {
  NOT_FOUND:                46001,
  NAME_INVALID:             46002,
  NAME_EXISTS:              46003,
  NAME_CONFLICT_WITH_FIELD: 46004,
  TYPE_INVALID:             46005,
  GROUP_NAME_INVALID:       46006,
  HAS_REFS:                 46007,
  DELETE_NOT_DISABLED:      46008,
  EDIT_NOT_DISABLED:        46009,
  VERSION_CONFLICT:         46010,
  DISABLED_REF:             46011,
} as const

/** 类型 4 枚举（前端下拉用；与 Server keys.go 泛型参数对齐） */
export const RUNTIME_BB_KEY_TYPES = [
  { value: 'integer', label: 'integer' },
  { value: 'float',   label: 'float' },
  { value: 'string',  label: 'string' },
  { value: 'bool',    label: 'bool' },
] as const

/** 分组 11 枚举（前端下拉用；与 Server keys.go 分节注释对齐） */
export const RUNTIME_BB_KEY_GROUPS = [
  { value: 'threat',   label: '威胁' },
  { value: 'event',    label: '事件' },
  { value: 'fsm',      label: 'FSM 状态' },
  { value: 'npc',      label: 'NPC 实例' },
  { value: 'action',   label: '行为追踪' },
  { value: 'need',     label: '需求' },
  { value: 'emotion',  label: '情绪' },
  { value: 'memory',   label: '记忆' },
  { value: 'social',   label: '社交' },
  { value: 'decision', label: '决策' },
  { value: 'move',     label: '移动' },
] as const

export const runtimeBbKeyApi = {
  list: (params: RuntimeBbKeyListQuery) =>
    request.post('/runtime-bb-keys/list', params) as Promise<ApiResponse<ListData<RuntimeBbKeyListItem>>>,
  create: (data: { name: string; type: string; label: string; description: string; group_name: string }) =>
    request.post('/runtime-bb-keys/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,
  detail: (id: number) =>
    request.post('/runtime-bb-keys/detail', { id }) as Promise<ApiResponse<RuntimeBbKey>>,
  update: (data: { id: number; type: string; label: string; description: string; group_name: string; version: number }) =>
    request.post('/runtime-bb-keys/update', data) as Promise<ApiResponse<string>>,
  delete: (id: number) =>
    request.post('/runtime-bb-keys/delete', { id }) as Promise<ApiResponse<DeleteResult>>,
  checkName: (name: string) =>
    request.post('/runtime-bb-keys/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,
  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/runtime-bb-keys/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
  references: (id: number) =>
    request.post('/runtime-bb-keys/references', { id }) as Promise<ApiResponse<RuntimeBbKeyReferenceDetail>>,
}
