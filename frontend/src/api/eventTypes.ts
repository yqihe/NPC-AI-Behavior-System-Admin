import request from './request'
import type { ApiResponse } from './request'

// ─── 类型定义 ───

/** 列表查询参数 */
export interface EventTypeListQuery {
  label?: string
  perception_mode?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

/** 列表项 */
export interface EventTypeListItem {
  id: number
  name: string
  display_name: string
  perception_mode: string
  enabled: boolean
  created_at: string
  default_severity: number
  default_ttl: number
  range: number
}

/** 列表响应 */
export interface EventTypeListData {
  items: EventTypeListItem[]
  total: number
  page: number
  page_size: number
}

/** 扩展字段 Schema（detail 接口返回） */
export interface ExtensionSchemaItem {
  field_name: string
  field_label: string
  field_type: string
  constraints: Record<string, unknown>
  default_value: unknown
  sort_order: number
}

/** 详情响应 */
export interface EventTypeDetail {
  id: number
  name: string
  display_name: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
  config: Record<string, unknown>
  extension_schema: ExtensionSchemaItem[]
}

/** 创建请求 */
export interface CreateEventTypeRequest {
  name: string
  display_name: string
  perception_mode: string
  default_severity: number
  default_ttl: number
  range: number
  extensions?: Record<string, unknown>
}

/** 编辑请求 */
export interface UpdateEventTypeRequest {
  id: number
  display_name: string
  perception_mode: string
  default_severity: number
  default_ttl: number
  range: number
  extensions: Record<string, unknown>
  version: number
}

/** 名称校验结果 */
export interface CheckNameResult {
  available: boolean
  message: string
}

// ─── 错误码（与 backend/internal/errcode/codes.go 42001-42015 保持一致）───

export const EVENT_TYPE_ERR = {
  NAME_EXISTS:         42001,
  NAME_INVALID:        42002,
  MODE_INVALID:        42003,
  SEVERITY_INVALID:    42004,
  TTL_INVALID:         42005,
  RANGE_INVALID:       42006,
  EXT_VALUE_INVALID:   42007,
  REF_DELETE:          42008,
  VERSION_CONFLICT:    42010,
  NOT_FOUND:           42011,
  DELETE_NOT_DISABLED: 42012,
  EDIT_NOT_DISABLED:   42015,
} as const

// ─── API 函数 ───

export const eventTypeApi = {
  list: (params: EventTypeListQuery) =>
    request.post('/event-types/list', params) as Promise<ApiResponse<EventTypeListData>>,

  create: (data: CreateEventTypeRequest) =>
    request.post('/event-types/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,

  detail: (id: number) =>
    request.post('/event-types/detail', { id }) as Promise<ApiResponse<EventTypeDetail>>,

  update: (data: UpdateEventTypeRequest) =>
    request.post('/event-types/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/event-types/delete', { id }) as Promise<ApiResponse<string>>,

  checkName: (name: string) =>
    request.post('/event-types/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/event-types/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
