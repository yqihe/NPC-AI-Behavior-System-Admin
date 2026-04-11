import request from './request'
import type { ApiResponse } from './request'
import type { ListData, CheckNameResult } from './fields'

// ============================================================================
// 类型定义（与 backend/internal/model/template.go 的 JSON tag 严格对齐）
// ============================================================================

/** 模板字段单元（templates.fields JSON 数组的元素） */
export interface TemplateFieldEntry {
  field_id: number
  required: boolean
}

/** 列表项（覆盖索引返回，不含 fields/description/version，toggle 前需调 detail 拿 version） */
export interface TemplateListItem {
  id: number
  name: string
  label: string
  ref_count: number
  enabled: boolean
  created_at: string
}

/** 详情中的字段精简信息 */
export interface TemplateFieldItem {
  field_id: number
  name: string
  label: string
  type: string
  category: string
  category_label: string
  /** 字段当前是否启用（停用字段在 UI 标灰 + 警告图标） */
  enabled: boolean
  /** 模板里的必填配置 */
  required: boolean
}

/** 详情（handler 层拼装，不进缓存） */
export interface TemplateDetail {
  id: number
  name: string
  label: string
  description: string
  enabled: boolean
  version: number
  ref_count: number
  created_at: string
  updated_at: string
  /** 顺序即 templates.fields JSON 数组顺序 */
  fields: TemplateFieldItem[]
}

/** 列表查询参数（enabled 三态：null 不筛选 / true 仅启用 / false 仅停用） */
export interface TemplateListQuery {
  label?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

export interface CreateTemplateRequest {
  name: string
  label: string
  description: string
  fields: TemplateFieldEntry[]
}

/** 编辑请求（无 name，name 创建后不可变） */
export interface UpdateTemplateRequest {
  id: number
  label: string
  description: string
  fields: TemplateFieldEntry[]
  version: number
}

/** NPC 引用方（NPC 模块未上线前 npcs 恒为空数组 make 生成） */
export interface TemplateReferenceItem {
  npc_id: number
  npc_name: string
}

export interface TemplateReferenceDetail {
  template_id: number
  template_label: string
  npcs: TemplateReferenceItem[]
}

// ============================================================================
// 错误码常量（41001-41012，与 backend/internal/errcode/codes.go 保持一致）
// ============================================================================

export const TEMPLATE_ERR = {
  NAME_EXISTS:         41001,
  NAME_INVALID:        41002,
  NOT_FOUND:           41003,
  NO_FIELDS:           41004,
  FIELD_DISABLED:      41005,
  FIELD_NOT_FOUND:     41006,
  REF_DELETE:          41007,
  REF_EDIT_FIELDS:     41008,
  DELETE_NOT_DISABLED: 41009,
  EDIT_NOT_DISABLED:   41010,
  VERSION_CONFLICT:    41011,
  FIELD_IS_REFERENCE:  41012,
} as const

/** 错误码中文文案映射 */
export const TEMPLATE_ERR_MSG: Record<number, string> = {
  [TEMPLATE_ERR.NAME_EXISTS]:         '模板标识已存在',
  [TEMPLATE_ERR.NAME_INVALID]:        '模板标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线',
  [TEMPLATE_ERR.NOT_FOUND]:           '模板不存在',
  [TEMPLATE_ERR.NO_FIELDS]:           '请至少勾选一个字段',
  [TEMPLATE_ERR.FIELD_DISABLED]:      '勾选的字段已停用，请先在字段管理中启用',
  [TEMPLATE_ERR.FIELD_NOT_FOUND]:     '勾选的字段不存在',
  [TEMPLATE_ERR.REF_DELETE]:          '该模板正被 NPC 引用，无法删除',
  [TEMPLATE_ERR.REF_EDIT_FIELDS]:     '该模板已被 NPC 引用，字段勾选与必填配置不可修改',
  [TEMPLATE_ERR.DELETE_NOT_DISABLED]: '请先停用该模板再删除',
  [TEMPLATE_ERR.EDIT_NOT_DISABLED]:   '请先停用该模板再编辑',
  [TEMPLATE_ERR.VERSION_CONFLICT]:    '该模板已被其他人修改，请刷新后重试',
  [TEMPLATE_ERR.FIELD_IS_REFERENCE]:  'reference 字段必须先展开子字段再加入模板',
}

// ============================================================================
// API 函数
// ============================================================================

export const templateApi = {
  list: (params: TemplateListQuery) =>
    request.post('/templates/list', params) as Promise<ApiResponse<ListData<TemplateListItem>>>,
  create: (data: CreateTemplateRequest) =>
    request.post('/templates/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,
  detail: (id: number) =>
    request.post('/templates/detail', { id }) as Promise<ApiResponse<TemplateDetail>>,
  update: (data: UpdateTemplateRequest) =>
    request.post('/templates/update', data) as Promise<ApiResponse<string>>,
  delete: (id: number) =>
    request.post('/templates/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,
  checkName: (name: string) =>
    request.post('/templates/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,
  references: (id: number) =>
    request.post('/templates/references', { id }) as Promise<ApiResponse<TemplateReferenceDetail>>,
  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/templates/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
