import request from './request'
import type { ApiResponse } from './request'
import type { ListData, CheckNameResult } from './fields'
import { dictApi, type DictionaryItem } from './dictionaries'

// ─── Spawn 子结构（对齐后端 model/region.go） ───

/** 2D 坐标（无 y 维度，对齐 Server zone.go Position{X,Z}） */
export interface SpawnPoint {
  x: number
  z: number
}

/** spawn_table 单条。respawn_seconds 本期 Server 不消费（v3 roadmap 占位） */
export interface SpawnEntry {
  template_ref: string
  count: number
  spawn_points: SpawnPoint[]
  wander_radius: number
  respawn_seconds: number
}

// ─── 列表 ───

export interface RegionListQuery {
  region_id?: string
  display_name?: string
  region_type?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

export interface RegionListItem {
  id: number
  region_id: string
  display_name: string
  region_type: string
  enabled: boolean
  created_at: string
}

// ─── 详情 ───

export interface RegionDetail {
  id: number
  region_id: string
  display_name: string
  region_type: string
  spawn_table: SpawnEntry[]
  enabled: boolean
  version: number
}

/** Region 语义别名（= RegionDetail），对齐后端 model.Region 外部引用 */
export type Region = RegionDetail

// ─── 请求 ───

export interface CreateRegionRequest {
  region_id: string
  display_name: string
  region_type: string
  spawn_table: SpawnEntry[]
}

export interface UpdateRegionRequest {
  id: number
  version: number
  display_name: string
  region_type: string
  spawn_table: SpawnEntry[]
}

// Re-export for consumers
export type { CheckNameResult } from './fields'

// ─── 错误码（对应 errcode/codes.go 47001–47011） ───

export const REGION_ERR = {
  ID_EXISTS:             47001,
  ID_INVALID:            47002,
  NOT_FOUND:             47003,
  TYPE_INVALID:          47004,
  SPAWN_ENTRY_INVALID:   47005,
  TEMPLATE_REF_NOT_FOUND: 47006,
  TEMPLATE_REF_DISABLED: 47007,
  DELETE_NOT_DISABLED:   47008,
  EDIT_NOT_DISABLED:     47009,
  VERSION_CONFLICT:      47010,
  EXPORT_DANGLING_REF:   47011,
} as const

// ─── API 对象 ───

export const regionApi = {
  list: (params: RegionListQuery) =>
    request.post('/regions/list', params) as Promise<ApiResponse<ListData<RegionListItem>>>,

  create: (data: CreateRegionRequest) =>
    request.post('/regions/create', data) as Promise<ApiResponse<{ id: number; region_id: string }>>,

  detail: (id: number) =>
    request.post('/regions/detail', { id }) as Promise<ApiResponse<RegionDetail>>,

  update: (data: UpdateRegionRequest) =>
    request.post('/regions/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/regions/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,

  checkName: (name: string) =>
    request.post('/regions/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/regions/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,

  /**
   * 拉 region_type 字典（wilderness / town 2 枚举）。
   * 委托 dictApi.list('region_type')，对齐字典组名 util.DictGroupRegionType。
   */
  getRegionTypeOptions: () => dictApi.list('region_type') as Promise<ApiResponse<{ items: DictionaryItem[] }>>,
}
