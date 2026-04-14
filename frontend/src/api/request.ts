import axios from 'axios'
import { ElMessage } from 'element-plus'

/** 后端统一响应格式 */
export interface ApiResponse<T = unknown> {
  code: number
  data: T
  message: string
}

/** 携带 code 的业务错误 */
export interface BizError extends Error {
  code: number
  data?: unknown
}

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 10000,
})

// 响应拦截器：统一业务错误处理
request.interceptors.response.use(
  (response) => {
    const { code, message } = response.data as ApiResponse
    if (code !== 0) {
      ElMessage.error(message || '操作失败')
      const err = new Error(message) as BizError
      err.code = code
      err.data = (response.data as ApiResponse).data
      return Promise.reject(err)
    }
    return response.data
  },
  (error) => {
    const msg = error.response?.data?.message || '网络错误，请检查后端服务'
    ElMessage.error(msg)
    return Promise.reject(error)
  },
)

export default request
