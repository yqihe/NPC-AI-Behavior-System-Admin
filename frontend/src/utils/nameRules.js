/**
 * 通用名称校验规则工厂。
 *
 * @param {object} opts
 * @param {Function} opts.listApi  - 返回 { data: { items: [...] } } 的 API 方法
 * @param {string}  opts.label     - 字段中文名（用于错误提示）
 * @param {boolean} opts.allowSlash - 是否允许 "/"（行为树名称需要���
 * @returns {Array} Element Plus 表单校验规则数组
 */
export function createNameRules({ listApi, label = '名称', allowSlash = false } = {}) {
  const pattern = allowSlash ? /^[a-z][a-z0-9_/]*$/ : /^[a-z][a-z0-9_]*$/
  const formatHint = allowSlash
    ? '以小写字母开头，只能包含小写字母、数字、下划线和斜杠'
    : '以小写字母开头，只能包含小写字母、数字和下划线'

  const rules = [
    { required: true, message: `请输入${label}`, trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (!value) return callback()
        if (!pattern.test(value)) {
          return callback(new Error(formatHint))
        }
        callback()
      },
      trigger: 'blur',
    },
  ]

  // 异步重复检测（仅在提供了 listApi 时启用）
  if (listApi) {
    rules.push({
      validator: async (_rule, value, callback) => {
        if (!value) return callback()
        try {
          const res = await listApi()
          const existing = (res.data.items || []).map(item => item.name)
          if (existing.includes(value)) {
            return callback(new Error(`${label} "${value}" 已存在，请换一个`))
          }
        } catch {
          // API 调用失败不阻塞用户，后端还有兜底校验
        }
        callback()
      },
      trigger: 'blur',
    })
  }

  return rules
}
