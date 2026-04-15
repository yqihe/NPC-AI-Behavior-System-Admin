import request from './request'
import type { ApiResponse } from './request'
import type { ListData, CheckNameResult } from './fields'

// ─── 条件树节点（对齐后端 model.FsmCondition）───

/** 条件树节点（递归，叶节点或组合节点） */
export interface FsmConditionNode {
  // 叶节点字段
  key?: string
  op?: string
  value?: unknown          // 直接值（number | string | boolean）
  ref_key?: string         // 引用 BB Key

  // 组合节点字段
  and?: FsmConditionNode[]
  or?: FsmConditionNode[]
}

// ─── 子结构 ───

/** 状态定义 */
export interface FsmState {
  name: string
}

/** 转换规则 */
export interface FsmTransition {
  from: string
  to: string
  priority: number
  condition: FsmConditionNode
}

// ─── 列表 ───

/** 列表查询参数 */
export interface FsmConfigListQuery {
  label?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

/** 列表项（initial_state/state_count 由后端 service 从 config_json 填充） */
export interface FsmConfigListItem {
  id: number
  name: string
  display_name: string
  initial_state: string
  initial_state_label: string
  state_count: number
  enabled: boolean
  created_at: string
}

// ─── 详情 ───

/** 详情接口响应（config 字段为展开的 config_json） */
export interface FsmConfigDetail {
  id: number
  name: string
  display_name: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
  config: FsmConfigBody
}

/** config_json 展开结构 */
export interface FsmConfigBody {
  initial_state: string
  states: FsmState[]
  transitions: FsmTransition[]
}

// ─── 请求 ───

/** 创建请求 */
export interface CreateFsmConfigRequest {
  name: string
  display_name: string
  initial_state: string
  states: FsmState[]
  transitions: FsmTransition[]
}

/** 编辑请求（name 不可变，无 name 字段） */
export interface UpdateFsmConfigRequest {
  id: number
  display_name: string
  initial_state: string
  states: FsmState[]
  transitions: FsmTransition[]
  version: number
}

// Re-export for consumers
export type { CheckNameResult } from './fields'

// ─── 错误码（与 backend/internal/errcode/codes.go 43001–43012 保持一致）───

export const FSM_ERR = {
  NAME_EXISTS:         43001,
  NAME_INVALID:        43002,
  NOT_FOUND:           43003,
  STATES_EMPTY:        43004,
  STATE_NAME_INVALID:  43005,
  INITIAL_INVALID:     43006,
  TRANSITION_INVALID:  43007,
  CONDITION_INVALID:   43008,
  DELETE_NOT_DISABLED: 43009,
  EDIT_NOT_DISABLED:   43010,
  VERSION_CONFLICT:    43011,
} as const

// ─── API 函数 ───

export const fsmConfigApi = {
  list: (params: FsmConfigListQuery) =>
    request.post('/fsm-configs/list', params) as Promise<ApiResponse<ListData<FsmConfigListItem>>>,

  create: (data: CreateFsmConfigRequest) =>
    request.post('/fsm-configs/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,

  detail: (id: number) =>
    request.post('/fsm-configs/detail', { id }) as Promise<ApiResponse<FsmConfigDetail>>,

  update: (data: UpdateFsmConfigRequest) =>
    request.post('/fsm-configs/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/fsm-configs/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,

  checkName: (name: string) =>
    request.post('/fsm-configs/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/fsm-configs/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
