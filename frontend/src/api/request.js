import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 10000,
})

// 响应拦截器：统一业务错误处理
request.interceptors.response.use(
  (response) => {
    const { code, message } = response.data
    if (code !== 0) {
      ElMessage.error(message || '操作失败')
      const err = new Error(message)
      err.code = code
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
