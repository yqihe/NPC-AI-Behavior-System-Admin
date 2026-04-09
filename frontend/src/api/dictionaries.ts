import request from './request'
import type { ApiResponse } from './request'

export interface DictionaryItem {
  name: string
  label: string
  extra?: Record<string, unknown>
}

export interface DictListData {
  items: DictionaryItem[]
}

export const dictApi = {
  list: (group: string) =>
    request.post('/dictionaries', { group }) as Promise<ApiResponse<DictListData>>,
}
