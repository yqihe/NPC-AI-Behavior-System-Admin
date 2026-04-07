import request from './index'

/**
 * 通用 CRUD API 工厂函数。
 * 给定资源路径，返回标准的 CRUD 方法集。
 * @param {string} resource - API 资源路径（如 'npc-templates'）
 */
export function createApi(resource) {
  return {
    list: () => request.get(`/${resource}`),
    get: (name) => request.get(`/${resource}/${encodeURIComponent(name)}`),
    create: (data) => request.post(`/${resource}`, data),
    update: (name, data) => request.put(`/${resource}/${encodeURIComponent(name)}`, data),
    remove: (name) => request.delete(`/${resource}/${encodeURIComponent(name)}`),
  }
}

// 预定义实体 API
export const npcTemplateApi = createApi('npc-templates')
export const eventTypeApi = createApi('event-types')
export const fsmConfigApi = createApi('fsm-configs')
export const btTreeApi = createApi('bt-trees')
export const regionApi = createApi('regions')
