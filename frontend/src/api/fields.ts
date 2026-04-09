import request from './request'
import type { ApiResponse } from './request'

/** 字段列表查询参数 */
export interface FieldListQuery {
  label?: string
  type?: string
  category?: string
  enabled?: boolean | null
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
  ref_count: number
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
  ref_count: number
  enabled: boolean
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
}

export interface CheckNameResult {
  available: boolean
  message: string
}

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
