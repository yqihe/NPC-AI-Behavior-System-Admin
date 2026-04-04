import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 10000,
})

// 响应拦截器：统一错误处理
request.interceptors.response.use(
  (response) => response,
  (error) => {
    const msg = error.response?.data?.error || '网络错误，请检查后端服务'
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

export default request
