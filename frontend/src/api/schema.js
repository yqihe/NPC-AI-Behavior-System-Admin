import request from './index'

/**
 * 组件 Schema 只读 API。
 * 用于获取组件字段定义（由游戏服务端定义，ADMIN 存储渲染）。
 */
export const componentSchemaApi = {
  list: () => request.get('/component-schemas'),
  get: (name) => request.get(`/component-schemas/${encodeURIComponent(name)}`),
}

/**
 * NPC 预设只读 API。
 * 用于获取 NPC 预设模板定义（creature/scene/quest/custom）。
 */
export const npcPresetApi = {
  list: () => request.get('/npc-presets'),
  get: (name) => request.get(`/npc-presets/${encodeURIComponent(name)}`),
}
