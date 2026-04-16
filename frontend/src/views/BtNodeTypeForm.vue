<template>
  <div class="bt-node-type-form">
    <!-- Header -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/bt-node-types')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/bt-node-types')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ isView ? '查看节点类型' : isCreate ? '新建节点类型' : '编辑节点类型' }}
      </span>
    </div>

    <!-- Scroll area -->
    <div class="form-scroll">
      <div class="form-body">

        <!-- Builtin warning -->
        <el-alert
          v-if="isBuiltin && !isCreate"
          type="warning"
          :closable="false"
          show-icon
          title="内置节点类型不可编辑或删除"
          style="margin-bottom: 0"
        />

        <!-- Basic info card -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-blue"></span>
            <span class="title-text">基本信息</span>
          </div>
          <el-form
            ref="formRef"
            :model="form"
            :rules="rules"
            :disabled="isView || isBuiltin"
            label-width="120px"
            label-position="right"
          >
            <!-- type_name -->
            <el-form-item label="类型标识" prop="type_name">
              <template v-if="!isCreate || isView">
                <el-input :model-value="form.type_name" disabled style="width: 100%">
                  <template #prefix><el-icon><Lock /></el-icon></template>
                </el-input>
                <div class="field-warn">
                  <el-icon><WarningFilled /></el-icon>
                  类型标识创建后不可更改
                </div>
              </template>
              <template v-else>
                <el-input
                  v-model="form.type_name"
                  placeholder="如 check_bb_float（小写字母开头，仅含小写字母、数字、下划线）"
                  style="width: 100%"
                  @blur="checkNameUnique"
                />
                <div v-if="nameStatus === 'checking'" class="field-hint">
                  <el-icon class="is-loading"><Loading /></el-icon> 校验中...
                </div>
                <div v-else-if="nameStatus === 'available'" class="field-hint field-hint-success">
                  <el-icon><CircleCheck /></el-icon> 标识符可用
                </div>
                <div v-else-if="nameStatus === 'taken'" class="field-hint field-hint-error">
                  <el-icon><CircleClose /></el-icon> {{ nameMessage }}
                </div>
              </template>
            </el-form-item>

            <!-- category -->
            <el-form-item label="节点分类" prop="category">
              <template v-if="!isCreate || isView">
                <el-tag :type="categoryTag(form.category)" size="default">
                  {{ categoryLabel(form.category) }}
                </el-tag>
              </template>
              <template v-else>
                <el-radio-group v-model="form.category">
                  <el-radio value="composite">组合节点（多子节点）</el-radio>
                  <el-radio value="decorator">装饰节点（单子节点）</el-radio>
                  <el-radio value="leaf">叶子节点（无子节点）</el-radio>
                </el-radio-group>
              </template>
            </el-form-item>

            <!-- label -->
            <el-form-item label="中文标签" prop="label">
              <el-input v-model="form.label" placeholder="如 序列、取反、检查浮点" style="width: 100%" />
            </el-form-item>

            <!-- description -->
            <el-form-item label="描述">
              <el-input
                v-model="form.description"
                type="textarea"
                :rows="3"
                placeholder="可选描述"
                style="width: 100%"
              />
            </el-form-item>
          </el-form>
        </div>

        <!-- Param schema card -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-orange"></span>
            <span class="title-text">参数定义</span>
            <el-tag size="small" type="info" style="margin-left: 8px">
              {{ form.category === 'leaf' ? '叶子节点可定义参数' : '组合/装饰节点通常无参数' }}
            </el-tag>
          </div>
          <BtParamSchemaEditor
            v-model="paramDefs"
            :disabled="isView || isBuiltin"
          />
          <div v-if="schemaError" class="schema-error">
            <el-icon><WarningFilled /></el-icon>
            {{ schemaError }}
          </div>
        </div>

      </div>
    </div>

    <!-- Footer: hidden in view mode or for builtin types -->
    <div v-if="!isView && !isBuiltin" class="form-footer">
      <el-button @click="$router.push('/bt-node-types')">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import {
  ArrowLeft, Lock, WarningFilled, Loading,
  CircleCheck, CircleClose,
} from '@element-plus/icons-vue'
import BtParamSchemaEditor from '@/components/BtParamSchemaEditor.vue'
import { btNodeTypeApi, BT_NODE_TYPE_ERR } from '@/api/btNodeTypes'
import type { BtParamDef } from '@/api/btNodeTypes'
import type { BizError } from '@/api/request'

const route = useRoute()
const router = useRouter()
const isCreate = route.meta.isCreate as boolean
const isView = (route.meta.isView as boolean) || false

const formRef = ref<FormInstance>()
const submitting = ref(false)
const nameStatus = ref<'' | 'checking' | 'available' | 'taken'>('')
const nameMessage = ref('')
const version = ref(0)
const isBuiltin = ref(false)
const schemaError = ref('')

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
    { required: true, message: '请输入类型标识', trigger: 'blur' },
    {
      pattern: namePattern,
      message: '小写字母开头，仅含小写字母、数字、下划线',
      trigger: 'blur',
    },
  ],
  category: [
    { required: true, message: '请选择节点分类', trigger: 'change' },
  ],
  label: [
    { required: true, message: '请输入中文标签', trigger: 'blur' },
  ],
}

function categoryLabel(cat: string): string {
  const map: Record<string, string> = {
    composite: '组合节点',
    decorator: '装饰节点',
    leaf: '叶子节点',
  }
  return map[cat] ?? cat
}

function categoryTag(cat: string): '' | 'success' | 'warning' | 'info' {
  const map: Record<string, '' | 'success' | 'warning' | 'info'> = {
    composite: '',
    decorator: 'warning',
    leaf: 'success',
  }
  return map[cat] ?? 'info'
}

// ─── init ───

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
    isBuiltin.value = data.is_builtin
    paramDefs.value = data.param_schema?.params ?? []
  } catch (err: unknown) {
    if ((err as BizError).code === BT_NODE_TYPE_ERR.NOT_FOUND) {
      ElMessage.error('节点类型不存在')
      router.push('/bt-node-types')
    }
  }
}

// ─── name check ───

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

// ─── param schema validation ───

function validateParamSchema(): boolean {
  for (const p of paramDefs.value) {
    if (!p.name) {
      schemaError.value = '参数名不能为空'
      return false
    }
    if (!p.label) {
      schemaError.value = `参数 "${p.name}" 的中文标签不能为空`
      return false
    }
    if (p.type === 'select' && (!p.options || p.options.length === 0)) {
      schemaError.value = `参数 "${p.name}" 为枚举类型，至少需要添加一个选项`
      return false
    }
  }
  schemaError.value = ''
  return true
}

// ─── submit ───

async function handleSubmit() {
  if (!validateParamSchema()) return

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
      ElMessage.success('创建成功')
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
    if (
      bizErr.code === BT_NODE_TYPE_ERR.NAME_EXISTS ||
      bizErr.code === BT_NODE_TYPE_ERR.NAME_INVALID
    ) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.NOT_FOUND) {
      ElMessage.error('节点类型不存在')
      router.push('/bt-node-types')
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.EDIT_NOT_DISABLED) {
      ElMessage.warning('请先禁用该节点类型后再编辑')
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.BUILTIN_EDIT) {
      ElMessage.warning('内置节点类型不可编辑')
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.PARAM_SCHEMA_INVALID) {
      schemaError.value = bizErr.message
      return
    }
    if (bizErr.code === BT_NODE_TYPE_ERR.CATEGORY_INVALID) {
      ElMessage.error(bizErr.message)
      return
    }
    // Other errors: global interceptor handles toast
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.bt-node-type-form {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.field-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}
.field-hint-success { color: #67C23A; }
.field-hint-error   { color: #F56C6C; }

.field-warn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #E6A23C;
}

.schema-error {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  font-size: 13px;
  color: #F56C6C;
}
</style>
