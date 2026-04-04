import request from './index'

function encodeName(name) {
  return encodeURIComponent(name)
}

export function list() {
  return request.get('/bt-trees')
}

export function get(name) {
  return request.get(`/bt-trees/${encodeName(name)}`)
}

export function create(data) {
  return request.post('/bt-trees', data)
}

export function update(name, data) {
  return request.put(`/bt-trees/${encodeName(name)}`, data)
}

export function remove(name) {
  return request.delete(`/bt-trees/${encodeName(name)}`)
}
