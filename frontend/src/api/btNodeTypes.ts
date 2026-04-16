import request from './request'
import type { ApiResponse } from './request'
import type { ListData, CheckNameResult } from './fields'

// ─── 节点类型参数定义 ───

/** param_schema 中单条参数定义 */
export interface BtParamDef {
  name: string
  label: string
  type: 'bb_key' | 'string' | 'float' | 'integer' | 'bool' | 'select'
  required: boolean
  options?: string[]  // select 类型时有值
}

/** 节点类型元信息（树编辑器加载用，从 param_schema 解析） */
export interface BtNodeTypeMeta {
  id: number
  type_name: string
  category: 'composite' | 'decorator' | 'leaf'
  label: string
  params: BtParamDef[]
}

// ─── 列表 ───

export interface BtNodeTypeListQuery {
  type_name?: string
  category?: string   // '' | 'composite' | 'decorator' | 'leaf'
  enabled?: boolean | null
  page: number
  page_size: number
}

export interface BtNodeTypeListItem {
  id: number
  type_name: string
  category: string
  label: string
  is_builtin: boolean
  enabled: boolean
}

// ─── 详情 ───

export interface BtNodeTypeDetail {
  id: number
  type_name: string
  category: string
  label: string
  description: string
  param_schema: { params: BtParamDef[] }
  is_builtin: boolean
  enabled: boolean
  version: number
}

// ─── 请求 ───

export interface CreateBtNodeTypeRequest {
  type_name: string
  category: string
  label: string
  description: string
  param_schema: { params: BtParamDef[] }
}

export interface UpdateBtNodeTypeRequest {
  id: number
  version: number
  label: string
  description: string
  param_schema: { params: BtParamDef[] }
}

// Re-export for consumers
export type { CheckNameResult } from './fields'

// ─── 错误码（对应 errcode/codes.go 44016–44026） ───

export const BT_NODE_TYPE_ERR = {
  NAME_EXISTS:          44016,
  NAME_INVALID:         44017,
  NOT_FOUND:            44018,
  CATEGORY_INVALID:     44019,
  DELETE_NOT_DISABLED:  44020,
  EDIT_NOT_DISABLED:    44021,
  REF_DELETE:           44022,
  BUILTIN_DELETE:       44023,
  BUILTIN_EDIT:         44024,
  PARAM_SCHEMA_INVALID: 44025,
  VERSION_CONFLICT:     44026,
} as const

// ─── API 对象 ───

export const btNodeTypeApi = {
  list: (params: BtNodeTypeListQuery) =>
    request.get('/bt-node-types', { params }) as Promise<ApiResponse<ListData<BtNodeTypeListItem>>>,

  create: (data: CreateBtNodeTypeRequest) =>
    request.post('/bt-node-types', data) as Promise<ApiResponse<{ id: number; type_name: string }>>,

  detail: (id: number) =>
    request.post('/bt-node-types/detail', { id }) as Promise<ApiResponse<BtNodeTypeDetail>>,

  update: (data: UpdateBtNodeTypeRequest) =>
    request.post('/bt-node-types/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/bt-node-types/delete', { id }) as Promise<ApiResponse<{ id: number; type_name: string; label: string }>>,

  checkName: (name: string) =>
    request.post('/bt-node-types/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/bt-node-types/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
