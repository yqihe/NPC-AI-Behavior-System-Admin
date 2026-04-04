<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 16px">
      <el-button @click="$router.push('/bt-trees')">返回列表</el-button>
      <h2 style="margin-left: 12px">{{ isEdit ? '编辑行为树' : '新建行为树' }}</h2>
    </div>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="140px" style="max-width: 800px" v-loading="pageLoading">
      <el-form-item label="行为树名称" prop="name">
        <el-input v-model="form.name" :disabled="isEdit" placeholder="如 civilian/idle" />
        <div class="field-hint" v-if="!isEdit">
          格式：NPC类型/状态，如 civilian/idle 表示平民的空闲行为树。
          以小写字母开头，只能包含小写字母、数字、下划线和斜杠。
        </div>
        <div class="field-hint" v-else>名称创建后不可修改，如需改名请删除后重新创建</div>
      </el-form-item>

      <el-divider>节点编辑</el-divider>
      <div class="field-hint" style="margin: -8px 0 16px 0">
        行为树从根节点开始，自上而下执行。复合节点（顺序/选择/并行）包含多个子节点，装饰节点只有一个子节点，叶子节点执行具体操作。
      </div>

      <BtNodeEditor v-model="form.rootNode" :depth="0" />

      <el-form-item style="margin-top: 16px">
        <el-button type="primary" @click="handleSubmit" :loading="submitting">保存</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as btTreeApi from '@/api/btTree'
import BtNodeEditor from '@/components/BtNodeEditor.vue'
import { createNameRules } from '@/utils/nameRules'

const route = useRoute()
const router = useRouter()
const formRef = ref(null)
const pageLoading = ref(false)
const submitting = ref(false)

const routeName = computed(() => decodeURIComponent(route.params.name || ''))
const isEdit = computed(() => routeName.value && routeName.value !== 'new')

const form = ref({
  name: '',
  rootNode: { type: 'sequence', children: [] },
})

const rules = computed(() => ({
  name: isEdit.value
    ? [{ required: true, message: '请输入行为树名称', trigger: 'blur' }]
    : createNameRules({ listApi: btTreeApi.list, label: '行为树名称', allowSlash: true }),
}))

async function loadData() {
  if (!isEdit.value) return
  pageLoading.value = true
  try {
    const res = await btTreeApi.get(routeName.value)
    const config = typeof res.data.config === 'string' ? JSON.parse(res.data.config) : res.data.config
    form.value = {
      name: res.data.name,
      rootNode: config || { type: 'sequence', children: [] },
    }
  } catch { /* 拦截器已处理 */ } finally { pageLoading.value = false }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    const payload = {
      name: form.value.name,
      config: form.value.rootNode,
    }
    if (isEdit.value) {
      await btTreeApi.update(routeName.value, payload)
      ElMessage.success('更新成功')
    } else {
      await btTreeApi.create(payload)
      ElMessage.success('创建成功')
    }
    router.push('/bt-trees')
  } catch { /* 拦截器已处理 */ } finally { submitting.value = false }
}

onMounted(loadData)
</script>

<style scoped>
.field-hint {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}
</style>
