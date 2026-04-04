import request from './index'

export function list() {
  return request.get('/bt-trees')
}

export function get(name) {
  return request.get(`/bt-trees/${encodeURIComponent(name)}`)
}

export function create(data) {
  return request.post('/bt-trees', data)
}

export function update(name, data) {
  return request.put(`/bt-trees/${encodeURIComponent(name)}`, data)
}

export function remove(name) {
  return request.delete(`/bt-trees/${encodeURIComponent(name)}`)
}
