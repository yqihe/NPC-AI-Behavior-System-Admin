<template>
  <div class="bt-node-type-form">
    <!-- 顶部导航栏 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="router.push('/bt-node-types')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="router.push('/bt-node-types')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ isView ? '查看节点类型' : isCreate ? '新建节点类型' : '编辑节点类型' }}
      </span>
    </div>

    <!-- 滚动内容区 -->
    <div class="form-scroll">
      <div class="form-body">

        <!-- Card 1: 基本信息 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-blue"></span>
            <span class="title-text">基本信息</span>
          </div>

          <el-alert
            v-if="isBuiltinLocked"
            type="info"
            :closable="false"
            title="内置节点类型，只可查看"
            style="margin-bottom: 16px"
          />

          <el-form
            ref="formRef"
            :model="form"
            :rules="rules"
            :disabled="isView || isBuiltinLocked"
            label-width="120px"
            label-position="right"
          >
            <!-- 节点标识 -->
            <el-form-item label="节点标识" prop="type_name">
              <template v-if="!isCreate">
                <el-input :model-value="form.type_name" disabled style="width: 100%">
                  <template #prefix><el-icon><Lock /></el-icon></template>
                </el-input>
                <div class="field-hint">创建后不可修改</div>
              </template>
              <template v-else>
                <el-input
                  v-model="form.type_name"
                  placeholder="如 sequence"
                  style="width: 100%"
                  @blur="checkNameUnique"
                />
                <div class="field-hint">格式：小写字母开头，仅含小写字母、数字、下划线，如 sequence</div>
                <div v-if="nameStatus === 'checking'" class="field-hint">校验中...</div>
                <div v-else-if="nameStatus === 'available'" class="field-hint field-hint-success">标识符可用</div>
                <div v-else-if="nameStatus === 'taken'" class="field-hint field-hint-error">{{ nameMessage }}</div>
              </template>
            </el-form-item>

            <!-- 节点分类：创建后不可修改，el-form disabled 接管 isView/builtin，这里仅追加 !isCreate -->
            <el-form-item label="节点分类" prop="category">
              <el-select
                v-model="form.category"
                :disabled="!isCreate || isView || isBuiltinLocked"
                style="width: 100%"
                placeholder="请选择节点分类"
              >
                <el-option label="组合节点（composite）" value="composite" />
                <el-option label="装饰节点（decorator）" value="decorator" />
                <el-option label="叶子节点（leaf）" value="leaf" />
              </el-select>
            </el-form-item>

            <!-- 中文名称 -->
            <el-form-item label="中文名称" prop="label">
              <el-input
                v-model="form.label"
                :disabled="isView || isBuiltinLocked"
                placeholder="如 序列节点"
                style="width: 100%"
              />
            </el-form-item>

            <!-- 描述 -->
            <el-form-item label="描述">
              <el-input
                v-model="form.description"
                :disabled="isView || isBuiltinLocked"
                type="textarea"
                :rows="3"
                placeholder="可选描述"
                style="width: 100%"
              />
            </el-form-item>
          </el-form>
        </div>

        <!-- Card 2: 参数定义 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-orange"></span>
            <span class="title-text">参数定义</span>
          </div>
          <BtParamSchemaEditor
            ref="paramSchemaEditorRef"
            v-model="paramDefs"
            :disabled="isView || isBuiltinLocked"
          />
        </div>

      </div>
    </div>

    <!-- 底部操作栏（查看/内置模式隐藏） -->
    <div v-if="!isView && !isBuiltinLocked" class="form-footer">
      <el-button @click="router.push('/bt-node-types')">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { ArrowLeft, Lock } from '@element-plus/icons-vue'
import BtParamSchemaEditor from '@/components/BtParamSchemaEditor.vue'
import { btNodeTypeApi, BT_NODE_TYPE_ERR } from '@/api/btNodeTypes'
import type { BtParamDef } from '@/api/btNodeTypes'
import type { BizError } from '@/api/request'

const route = useRoute()
const router = useRouter()
const isCreate = route.meta.isCreate as boolean
const isView = (route.meta.isView as boolean) || false

const formRef = ref<FormInstance>()
const paramSchemaEditorRef = ref<InstanceType<typeof BtParamSchemaEditor>>()
const submitting = ref(false)
const nameStatus = ref<'' | 'checking' | 'available' | 'taken'>('')
const nameMessage = ref('')
const version = ref(0)
const isBuiltinLocked = ref(false)

const form = reactive({
  type_name: '',
  category: 'leaf' as 'composite' | 'decorator' | 'leaf',
  label: '',
  description: '',
})

const paramDefs = ref<BtParamDef[]>([])

const namePattern = /^[a-z][a-z0-9_]*$/

const rules = {
  type_name: [
    { required: true, message: '请输入节点标识', trigger: 'blur' },
    {
      pattern: namePattern,
      message: '格式：小写字母开头，仅含小写字母、数字、下划线',
      trigger: 'blur',
    },
  ],
  category: [
    { required: true, message: '请选择节点分类', trigger: 'change' },
  ],
  label: [
    { required: true, message: '请输入中文名称', trigger: 'blur' },
  ],
}

// ─── 初始化 ───

onMounted(async () => {
  if (!isCreate) await loadDetail()
})

async function loadDetail() {
  const id = Number(route.params.id)
  try {
    const res = await btNodeTypeApi.detail(id)
    const data = res.data
    form.type_name = data.type_name
    form.category = data.category as 'composite' | 'decorator' | 'leaf'
    form.label = data.label
    form.description = data.description ?? ''
    version.value = data.version
    isBuiltinLocked.value = data.is_builtin === true
    paramDefs.value = data.param_schema?.params ?? []
  } catch (err: unknown) {
    if ((err as BizError).code === BT_NODE_TYPE_ERR.NOT_FOUND) {
      ElMessage.error('节点类型不存在')
      router.push('/bt-node-types')
    }
  }
}

// ─── 标识符校验 ───

async function checkNameUnique() {
  if (!form.type_name || !namePattern.test(form.type_name)) {
    nameStatus.value = ''
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await btNodeTypeApi.checkName(form.type_name)
    if (res.data?.available) {
      nameStatus.value = 'available'
      nameMessage.value = ''
    } else {
      nameStatus.value = 'taken'
      nameMessage.value = res.data?.message || '标识符已被使用'
    }
  } catch {
    nameStatus.value = ''
  }
}

// ─── 提交 ───

async function handleSubmit() {
  const schemaErr = paramSchemaEditorRef.value?.validate()
  if (schemaErr) {
    ElMessage.error(schemaErr)
    return
  }

  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.warning('标识符已被使用，请更换')
    return
  }

  submitting.value = true
  try {
    if (isCreate) {
      await btNodeTypeApi.create({
        type_name: form.type_name,
        category: form.category,
        label: form.label,
        description: form.description,
        param_schema: { params: paramDefs.value },
      })
      ElMessage.success('创建成功，节点类型默认为禁用状态，确认无误后请手动启用')
    } else {
      await btNodeTypeApi.update({
        id: Number(route.params.id),
        version: version.value,
        label: form.label,
        description: form.description,
        param_schema: { params: paramDefs.value },
      })
      ElMessage.success('保存成功')
    }
    router.push('/bt-node-types')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === BT_NODE_TYPE_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他人修改，请刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.NAME_EXISTS || bizErr.code === BT_NODE_TYPE_ERR.NAME_INVALID) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.NOT_FOUND) {
      ElMessage.error('节点类型不存在')
      router.push('/bt-node-types')
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.BUILTIN_EDIT) {
      ElMessage.warning('内置节点类型不可编辑')
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.PARAM_SCHEMA_INVALID) {
      ElMessage.error(bizErr.message)
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.CATEGORY_INVALID) {
      ElMessage.error(bizErr.message)
      return
    }
    // 其他错误拦截器已 toast
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
/* 容器：撑满父级，flex 纵向 */
.bt-node-type-form {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 字段提示（组件私有） */
.field-hint {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
.field-hint-success { color: #67C23A; }
.field-hint-error   { color: #F56C6C; }
</style>
