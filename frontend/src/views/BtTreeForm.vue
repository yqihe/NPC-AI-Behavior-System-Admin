<template>
  <div class="bt-tree-form">
    <div class="form-header">
      <h2>{{ isEdit ? '编辑行为树' : '新建行为树' }}</h2>
      <el-button @click="goBack">返回列表</el-button>
    </div>

    <el-form
      ref="formRef"
      :model="formModel"
      :rules="nameFieldRules"
      label-position="top"
      style="max-width: 900px"
    >
      <el-form-item label="行为树名称" prop="name">
        <el-input
          v-model="formModel.name"
          :disabled="isEdit"
          placeholder="如 wolf/idle（允许斜杠分组）"
        />
      </el-form-item>

      <el-form-item label="行为树结构">
        <div v-if="nodeTypesLoaded" style="width: 100%">
          <bt-node-editor
            v-model="rootNode"
            :node-types="nodeTypes"
          />
        </div>
        <el-skeleton v-else :rows="3" animated />
      </el-form-item>

      <el-form-item>
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
import BtNodeEditor from '@/components/BtNodeEditor.vue'
import { btTreeApi } from '@/api/generic'
import { nodeTypeSchemaApi } from '@/api/schema'
import { createNameRules } from '@/utils/nameRules'

const route = useRoute()
const router = useRouter()

const routeName = route.params.name
const isEdit = computed(() => !!routeName && routeName !== 'new')

const formRef = ref(null)
const saving = ref(false)
const formModel = reactive({ name: '' })
const rootNode = ref({ type: 'sequence', children: [] })
const nodeTypes = ref([])
const nodeTypesLoaded = ref(false)

const nameFieldRules = computed(() => ({
  name: isEdit.value
    ? [{ required: true, message: '名称不能为空', trigger: 'blur' }]
    : createNameRules({ listApi: btTreeApi.list, label: '行为树', allowSlash: true }),
}))

onMounted(async () => {
  // 加载节点类型
  try {
    const res = await nodeTypeSchemaApi.list()
    nodeTypes.value = res.data.items || []
    nodeTypesLoaded.value = true
  } catch {
    ElMessage.error('加载节点类型失败')
  }

  // 编辑模式
  if (isEdit.value) {
    try {
      const res = await btTreeApi.get(routeName)
      formModel.name = res.data.name
      rootNode.value = res.data.config || { type: 'sequence', children: [] }
    } catch {
      ElMessage.error('加载行为树数据失败')
      goBack()
    }
  }
})

async function handleSave() {
  if (formRef.value) {
    try { await formRef.value.validate() } catch { return }
  }

  saving.value = true
  try {
    const payload = {
      name: formModel.name,
      config: rootNode.value,
    }

    if (isEdit.value) {
      await btTreeApi.update(routeName, payload)
      ElMessage.success('保存成功')
    } else {
      await btTreeApi.create(payload)
      ElMessage.success('创建成功')
    }
    goBack()
  } catch { /* 拦截器已处理 */ }
  finally { saving.value = false }
}

function goBack() {
  router.push('/bt-trees')
}
</script>

<style scoped>
.bt-tree-form {
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
