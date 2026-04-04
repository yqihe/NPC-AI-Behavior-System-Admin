import request from './index'

export function list() {
  return request.get('/npc-types')
}

export function get(name) {
  return request.get(`/npc-types/${name}`)
}

export function create(data) {
  return request.post('/npc-types', data)
}

export function update(name, data) {
  return request.put(`/npc-types/${name}`, data)
}

export function remove(name) {
  return request.delete(`/npc-types/${name}`)
}
