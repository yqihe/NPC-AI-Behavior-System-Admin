import request from './request'

export const dictApi = {
  list: (group) => request.post('/dictionaries', { group }),
}
