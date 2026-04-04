import request from './index'

export function list() {
  return request.get('/event-types')
}

export function get(name) {
  return request.get(`/event-types/${encodeURIComponent(name)}`)
}

export function create(data) {
  return request.post('/event-types', data)
}

export function update(name, data) {
  return request.put(`/event-types/${encodeURIComponent(name)}`, data)
}

export function remove(name) {
  return request.delete(`/event-types/${encodeURIComponent(name)}`)
}
