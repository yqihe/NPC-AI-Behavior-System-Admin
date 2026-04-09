import request from './request'

export const fieldApi = {
  list: (params) => request.post('/fields/list', params),
  create: (data) => request.post('/fields/create', data),
  detail: (id) => request.post('/fields/detail', { id }),
  update: (data) => request.post('/fields/update', data),
  delete: (id) => request.post('/fields/delete', { id }),
  checkName: (name) => request.post('/fields/check-name', { name }),
  toggleEnabled: (id, enabled, version) =>
    request.post('/fields/toggle-enabled', { id, enabled, version }),
  references: (id) => request.post('/fields/references', { id }),
}
