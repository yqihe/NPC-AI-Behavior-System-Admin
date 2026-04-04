import request from './index'

export function list() {
  return request.get('/fsm-configs')
}

export function get(name) {
  return request.get(`/fsm-configs/${encodeURIComponent(name)}`)
}

export function create(data) {
  return request.post('/fsm-configs', data)
}

export function update(name, data) {
  return request.put(`/fsm-configs/${encodeURIComponent(name)}`, data)
}

export function remove(name) {
  return request.delete(`/fsm-configs/${encodeURIComponent(name)}`)
}
