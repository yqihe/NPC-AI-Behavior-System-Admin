<template>
  <div class="generic-form">
    <div class="form-header">
      <h2>{{ isEdit ? `编辑 ${title}` : `新建 ${title}` }}</h2>
      <el-button @click="goBack">返回列表</el-button>
    </div>

    <el-form
      ref="formRef"
      :model="formModel"
      :rules="nameFieldRules"
      label-position="top"
      style="max-width: 800px"
    >
      <!-- 名称字段 -->
      <el-form-item label="名称" prop="name">
        <el-input
          v-model="formModel.name"
          :disabled="isEdit"
          placeholder="请输入名称（如 wolf_common）"
        />
      </el-form-item>

      <!-- 配置字段（SchemaForm 渲染） -->
      <el-form-item label="配置">
        <schema-form
          v-model="formModel.config"
          :schema="configSchema"
          @submit="handleSchemaSubmit"
        />
      </el-form-item>

      <!-- 无 schema 时的保存按钮（有 schema 时 SchemaForm 内部有保存按钮） -->
      <el-form-item v-if="!configSchema">
        <el-button type="primary" :loading="saving" @click="handleSave">
          保存
        </el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import SchemaForm from '@/components/SchemaForm.vue'
import { createNameRules } from '@/utils/nameRules'
import { componentSchemaApi } from '@/api/schema'

const route = useRoute()
const router = useRouter()

const title = route.meta?.title || '配置'
const entityPath = route.meta?.entityPath || ''
const api = route.meta?.api
const allowSlash = route.meta?.allowSlash || false

// schema：优先静态传入，其次按 schemaName 异步加载
const staticSchema = route.meta?.configSchema || null
const schemaName = route.meta?.schemaName || null
const configSchema = ref(staticSchema)

// 编辑模式判断
const routeName = route.params.name
const isEdit = computed(() => !!routeName && routeName !== 'new')

const formRef = ref(null)
const saving = ref(false)

const formModel = reactive({
  name: '',
  config: {},
})

// name 校验规则（新建时检查重复）
const nameFieldRules = computed(() => ({
  name: isEdit.value
    ? [{ required: true, message: '名称不能为空', trigger: 'blur' }]
    : createNameRules({ listApi: api?.list, label: title, allowSlash }),
}))

onMounted(async () => {
  // 按 schemaName 异步加载 schema（如果没有静态传入）
  if (!configSchema.value && schemaName) {
    try {
      const res = await componentSchemaApi.get(schemaName)
      configSchema.value = res.data.config?.schema || null
    } catch { /* schema 加载失败则降级为 JSON 编辑器 */ }
  }

  // 编辑模式：加载已有数据
  if (isEdit.value && api) {
    try {
      const res = await api.get(routeName)
      formModel.name = res.data.name
      formModel.config = res.data.config || {}
    } catch {
      ElMessage.error('加载数据失败')
      goBack()
    }
  }
})

// SchemaForm 提交（有 schema 时）
function handleSchemaSubmit(configData) {
  formModel.config = configData
  handleSave()
}

// 保存
async function handleSave() {
  // 先校验 name
  if (formRef.value) {
    try {
      await formRef.value.validate()
    } catch {
      return // 校验未通过
    }
  }

  if (!formModel.name.trim()) {
    ElMessage.warning('请输入名称')
    return
  }

  saving.value = true
  try {
    const payload = {
      name: formModel.name,
      config: formModel.config || {},
    }

    if (isEdit.value) {
      await api.update(routeName, payload)
      ElMessage.success('保存成功')
    } else {
      await api.create(payload)
      ElMessage.success('创建成功')
    }
    goBack()
  } catch { /* 拦截器已处理 */ }
  finally { saving.value = false }
}

function goBack() {
  router.push(`/${entityPath}`)
}
</script>

<style scoped>
.generic-form {
  padding: 24px;
}
.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.form-header h2 {
  margin: 0;
  color: #303133;
  font-size: 20px;
}
</style>
