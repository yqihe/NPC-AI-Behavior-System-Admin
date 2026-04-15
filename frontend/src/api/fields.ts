import request from './request'
import type { ApiResponse } from './request'

/** 字段列表查询参数 */
export interface FieldListQuery {
  label?: string
  type?: string
  category?: string
  enabled?: boolean | null
  bb_exposed?: boolean      // 仅返回 expose_bb=true 的字段（BBKeySelector 用）
  page: number
  page_size: number
}

/** 字段列表项 */
export interface FieldListItem {
  id: number
  name: string
  label: string
  type: string
  category: string
  enabled: boolean
  created_at: string
  type_label: string
  category_label: string
  version: number
}

/** 字段属性 */
export interface FieldProperties {
  description?: string
  expose_bb: boolean
  default_value?: unknown
  constraints?: Record<string, unknown>
}

/** 字段详情 */
export interface FieldDetail {
  id: number
  name: string
  label: string
  type: string
  category: string
  properties: FieldProperties
  enabled: boolean
  has_refs: boolean
  version: number
  created_at: string
  updated_at: string
}

/** 列表响应 */
export interface ListData<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

/** 引用详情 */
export interface ReferenceItem {
  ref_type: string
  ref_id: number
  label: string
}

export interface ReferenceDetail {
  field_id: number
  field_label: string
  templates: ReferenceItem[]
  fields: ReferenceItem[]
  fsms: ReferenceItem[]
}

export interface CheckNameResult {
  available: boolean
  message: string
}

// 字段管理段错误码（40001-40017，与 backend/internal/errcode/codes.go 保持一致）
export const FIELD_ERR = {
  NAME_EXISTS:         40001,
  NAME_INVALID:        40002,
  TYPE_NOT_FOUND:      40003,
  CATEGORY_NOT_FOUND:  40004,
  REF_DELETE:          40005,
  REF_CHANGE_TYPE:     40006,
  REF_TIGHTEN:         40007,
  BB_KEY_IN_USE:       40008,
  CYCLIC_REF:          40009,
  VERSION_CONFLICT:    40010,
  NOT_FOUND:           40011,
  DELETE_NOT_DISABLED: 40012,
  REF_DISABLED:        40013,
  REF_NOT_FOUND:       40014,
  EDIT_NOT_DISABLED:   40015,
  REF_NESTED:          40016,
  REF_EMPTY:           40017,
} as const

export const fieldApi = {
  list: (params: FieldListQuery) =>
    request.post('/fields/list', params) as Promise<ApiResponse<ListData<FieldListItem>>>,
  create: (data: { name: string; label: string; type: string; category: string; properties: FieldProperties }) =>
    request.post('/fields/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,
  detail: (id: number) =>
    request.post('/fields/detail', { id }) as Promise<ApiResponse<FieldDetail>>,
  update: (data: { id: number; label: string; type: string; category: string; properties: FieldProperties; version: number }) =>
    request.post('/fields/update', data) as Promise<ApiResponse<string>>,
  delete: (id: number) =>
    request.post('/fields/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,
  checkName: (name: string) =>
    request.post('/fields/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,
  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/fields/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
  references: (id: number) =>
    request.post('/fields/references', { id }) as Promise<ApiResponse<ReferenceDetail>>,
}
