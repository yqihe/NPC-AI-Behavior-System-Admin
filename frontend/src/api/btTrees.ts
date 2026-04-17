import request from './request'
import type { ApiResponse } from './request'
import type { ListData, CheckNameResult } from './fields'
// ─── 编辑器内部节点结构 ───

/** 编辑器内部节点表示（params 单独存，序列化时展开到顶层） */
export interface BtNodeInternal {
  type: string
  category: 'composite' | 'decorator' | 'leaf'
  params: Record<string, unknown>
  children?: BtNodeInternal[]    // composite 用
  child?: BtNodeInternal | null  // decorator 用
}

// ─── 列表 ───

export interface BtTreeListQuery {
  name?: string
  display_name?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

export interface BtTreeListItem {
  id: number
  name: string
  display_name: string
  enabled: boolean
  created_at: string
}

// ─── 详情 ───

export interface BtTreeDetail {
  id: number
  name: string
  display_name: string
  description: string
  config: unknown
  enabled: boolean
  version: number
}

// ─── 请求 ───

export interface CreateBtTreeRequest {
  name: string
  display_name: string
  description: string
  config: unknown
}

export interface UpdateBtTreeRequest {
  id: number
  version: number
  display_name: string
  description: string
  config: unknown
}

// Re-export for consumers
export type { CheckNameResult } from './fields'

// ─── 错误码（对应 errcode/codes.go 44001–44012） ───

export const BT_TREE_ERR = {
  NAME_EXISTS:         44001,
  NAME_INVALID:        44002,
  NOT_FOUND:           44003,
  CONFIG_INVALID:      44004,
  NODE_TYPE_NOT_FOUND: 44005,
  DEPTH_EXCEEDED:      44006,
  DELETE_NOT_DISABLED: 44009,
  EDIT_NOT_DISABLED:   44010,
  VERSION_CONFLICT:    44011,
  REF_DELETE:          44012,
} as const

// ─── 序列化 / 反序列化 ───

import type { BtNodeTypeMeta } from './btNodeTypes'

/**
 * 将编辑器内部结构序列化为后端 config JSON。
 * params 展开到节点顶层，不加 params 包装层。
 */
export function serializeBtNode(node: BtNodeInternal): Record<string, unknown> {
  const out: Record<string, unknown> = { type: node.type, ...node.params }
  if (node.category === 'composite') {
    out.children = (node.children ?? []).map(serializeBtNode)
  } else if (node.category === 'decorator') {
    out.child = node.child ? serializeBtNode(node.child) : null
  }
  return out
}

/**
 * 将后端 config JSON 反序列化为编辑器内部结构。
 * typeMap 中找不到的类型：category 降级 'leaf'，params 保留全部 key-value（静默降级）。
 */
export function deserializeBtNode(
  json: Record<string, unknown>,
  typeMap: Map<string, BtNodeTypeMeta>,
): BtNodeInternal {
  const typeName = json['type'] as string
  const children = json['children']
  const child = json['child']
  const meta = typeMap.get(typeName)
  const category = meta?.category ?? 'leaf'
  const paramNames = new Set((meta?.params ?? []).map((p) => p.name))

  const params: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(json)) {
    if (k === 'type' || k === 'children' || k === 'child') continue
    // 已知 meta 时只取 paramNames 内的 key；未知类型时保留全部（不丢数据）
    if (!meta || paramNames.has(k)) {
      params[k] = v
    }
  }

  const node: BtNodeInternal = { type: typeName, category, params }

  if (category === 'composite' && Array.isArray(children)) {
    node.children = children.map((c) =>
      deserializeBtNode(c as Record<string, unknown>, typeMap),
    )
  }
  if (category === 'decorator' && child && typeof child === 'object') {
    node.child = deserializeBtNode(child as Record<string, unknown>, typeMap)
  }

  return node
}

// ─── API 对象 ───

export const btTreeApi = {
  list: (params: BtTreeListQuery) =>
    request.post('/bt-trees/list', params) as Promise<ApiResponse<ListData<BtTreeListItem>>>,

  create: (data: CreateBtTreeRequest) =>
    request.post('/bt-trees/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,

  detail: (id: number) =>
    request.post('/bt-trees/detail', { id }) as Promise<ApiResponse<BtTreeDetail>>,

  update: (data: UpdateBtTreeRequest) =>
    request.post('/bt-trees/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/bt-trees/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,

  checkName: (name: string) =>
    request.post('/bt-trees/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/bt-trees/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,

  references: (id: number) =>
    request.post('/bt-trees/references', { id }) as Promise<ApiResponse<BtTreeReferenceDetail>>,
}

export interface BtTreeReferenceNPC {
  npc_id: number
  npc_name: string
  npc_label: string
}

export interface BtTreeReferenceDetail {
  bt_tree_id: number
  bt_tree_label: string
  npcs: BtTreeReferenceNPC[]
}
