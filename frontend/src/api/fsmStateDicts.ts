import request from './request'
import type { ApiResponse } from './request'
import type { ListData, CheckNameResult } from './fields'

// ─── 类型定义 ───

/** 列表查询参数 */
export interface FsmStateDictListQuery {
  name?: string
  category?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

/** 列表项 */
export interface FsmStateDictListItem {
  id: number
  name: string
  display_name: string
  category: string
  enabled: boolean
  created_at: string
}

/** 完整详情（detail 接口返回） */
export interface FsmStateDict {
  id: number
  name: string
  display_name: string
  category: string
  description: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
}

/** 创建请求 */
export interface CreateFsmStateDictRequest {
  name: string
  display_name: string
  category: string
  description?: string
}

/** 编辑请求 */
export interface UpdateFsmStateDictRequest {
  id: number
  display_name: string
  category: string
  description: string
  version: number
}

/** FSM 配置引用信息（delete 返回 43020 时携带） */
export interface FsmConfigRef {
  id: number
  name: string
  display_name: string
  enabled: boolean
}

/** 删除结果 */
export interface FsmStateDictDeleteResult {
  id: number
  name: string
  display_name: string
  referenced_by: FsmConfigRef[]
}

// Re-export for consumers
export type { CheckNameResult } from './fields'

// ─── 错误码（与 backend/internal/errcode/codes.go 43013–43020 保持一致）───

export const FSM_STATE_DICT_ERR = {
  NAME_EXISTS:          43013,
  NAME_INVALID:         43014,
  NOT_FOUND:            43015,
  DELETE_NOT_DISABLED:  43016,
  VERSION_CONFLICT:     43017,
  IN_USE:               43020,
} as const

// ─── API 函数 ───

export const fsmStateDictApi = {
  list: (params: FsmStateDictListQuery) =>
    request.post('/fsm-state-dicts/list', params) as Promise<ApiResponse<ListData<FsmStateDictListItem>>>,

  create: (data: CreateFsmStateDictRequest) =>
    request.post('/fsm-state-dicts/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,

  detail: (id: number) =>
    request.post('/fsm-state-dicts/detail', { id }) as Promise<ApiResponse<FsmStateDict>>,

  update: (data: UpdateFsmStateDictRequest) =>
    request.post('/fsm-state-dicts/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/fsm-state-dicts/delete', { id }) as Promise<ApiResponse<FsmStateDictDeleteResult>>,

  checkName: (name: string) =>
    request.post('/fsm-state-dicts/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/fsm-state-dicts/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,

  listCategories: () =>
    request.post('/fsm-state-dicts/list-categories', {}) as Promise<ApiResponse<string[]>>,
}
