<template>
  <div class="bt-tree-form">
    <!-- Header -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/bt-trees')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/bt-trees')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ isView ? '查看行为树' : isCreate ? '新建行为树' : '编辑行为树' }}
      </span>
    </div>

    <!-- Scroll area -->
    <div class="form-scroll">
      <div class="form-body-wide">

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
            :disabled="isView"
            label-width="120px"
            label-position="right"
          >
            <!-- name -->
            <el-form-item label="行为树标识" prop="name">
              <template v-if="!isCreate || isView">
                <el-input :model-value="form.name" disabled style="width: 100%">
                  <template #prefix><el-icon><Lock /></el-icon></template>
                </el-input>
                <div class="field-warn">
                  <el-icon><WarningFilled /></el-icon>
                  标识符创建后不可更改
                </div>
              </template>
              <template v-else>
                <el-input
                  v-model="form.name"
                  placeholder="如 wolf/attack（小写字母开头，仅含小写字母/数字/下划线/斜杠）"
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

            <!-- display_name -->
            <el-form-item label="中文名称" prop="display_name">
              <el-input
                v-model="form.display_name"
                placeholder="如 狼 — 攻击行为树"
                style="width: 100%"
              />
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

        <!-- Tree config card -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-green"></span>
            <span class="title-text">树结构</span>
          </div>

          <div v-if="loadingNodeTypes" class="loading-hint">
            <el-icon class="is-loading"><Loading /></el-icon>
            加载节点类型...
          </div>

          <template v-else>
            <!-- No root node -->
            <template v-if="!rootNode">
              <div v-if="!isView" class="no-root-area">
                <span class="no-root-hint">尚未设置根节点</span>
                <el-button type="primary" size="small" @click="showRootSelector = true">
                  设置根节点
                </el-button>
              </div>
              <div v-else class="no-root-hint">（空树）</div>
            </template>

            <!-- Root node editor -->
            <BtNodeEditor
              v-else
              :model-value="rootNode"
              :node-types="nodeTypeMetas"
              :disabled="isView"
              :depth="0"
              @update:model-value="rootNode = $event"
              @delete="rootNode = null"
            />

            <div v-if="treeError" class="tree-error">
              <el-icon><WarningFilled /></el-icon>
              {{ treeError }}
            </div>
          </template>
        </div>

      </div>
    </div>

    <!-- Footer -->
    <div v-if="!isView" class="form-footer">
      <el-button @click="$router.push('/bt-trees')">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
    </div>

    <!-- Root node type selector dialog -->
    <BtNodeTypeSelector
      v-model="showRootSelector"
      :node-types="nodeTypeMetas"
      @select="onRootTypeSelected"
    />
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
import BtNodeEditor from '@/components/BtNodeEditor.vue'
import BtNodeTypeSelector from '@/components/BtNodeTypeSelector.vue'
import {
  btTreeApi, BT_TREE_ERR,
  serializeBtNode, deserializeBtNode,
} from '@/api/btTrees'
import type { BtNodeInternal } from '@/api/btTrees'
import { btNodeTypeApi } from '@/api/btNodeTypes'
import type { BtNodeTypeMeta } from '@/api/btNodeTypes'
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
const treeError = ref('')

const form = reactive({
  name: '',
  display_name: '',
  description: '',
})

const rootNode = ref<BtNodeInternal | null>(null)
const showRootSelector = ref(false)

const loadingNodeTypes = ref(false)
const nodeTypeMetas = ref<BtNodeTypeMeta[]>([])

const namePattern = /^[a-z][a-z0-9_/]*$/

const rules = {
  name: [
    { required: true, message: '请输入行为树标识', trigger: 'blur' },
    {
      pattern: namePattern,
      message: '小写字母开头，仅含小写字母、数字、下划线、斜杠',
      trigger: 'blur',
    },
  ],
  display_name: [
    { required: true, message: '请输入中文名称', trigger: 'blur' },
  ],
}

// ─── init ───

onMounted(async () => {
  await loadNodeTypes()
  if (!isCreate) await loadDetail()
})

async function loadNodeTypes() {
  loadingNodeTypes.value = true
  try {
    const listRes = await btNodeTypeApi.list({ enabled: true, page: 1, page_size: 1000 })
    const items = listRes.data?.items ?? []

    // Fetch all details in parallel to obtain param_schema (not in list response)
    const detailResults = await Promise.allSettled(
      items.map((item) => btNodeTypeApi.detail(item.id)),
    )

    const metas: BtNodeTypeMeta[] = []
    for (let i = 0; i < items.length; i++) {
      const item = items[i]
      const result = detailResults[i]
      const params =
        result.status === 'fulfilled'
          ? (result.value.data?.param_schema?.params ?? [])
          : []
      metas.push({
        id: item.id,
        type_name: item.type_name,
        category: item.category as 'composite' | 'decorator' | 'leaf',
        label: item.label,
        params,
      })
    }
    nodeTypeMetas.value = metas
  } catch {
    // interceptor handles toast
  } finally {
    loadingNodeTypes.value = false
  }
}

async function loadDetail() {
  const id = Number(route.params.id)
  try {
    const res = await btTreeApi.detail(id)
    const data = res.data
    form.name = data.name
    form.display_name = data.display_name
    form.description = data.description ?? ''
    version.value = data.version

    const typeMap = new Map(nodeTypeMetas.value.map((m) => [m.type_name, m]))
    if (data.config && typeof data.config === 'object') {
      rootNode.value = deserializeBtNode(
        data.config as Record<string, unknown>,
        typeMap,
      )
    }
  } catch (err: unknown) {
    if ((err as BizError).code === BT_TREE_ERR.NOT_FOUND) {
      ElMessage.error('行为树不存在')
      router.push('/bt-trees')
    }
  }
}

// ─── root node selector ───

function makeDefaultNode(meta: BtNodeTypeMeta): BtNodeInternal {
  const params: Record<string, unknown> = {}
  for (const p of meta.params) {
    if (p.type === 'bool') params[p.name] = false
    else if (p.type === 'float' || p.type === 'integer') params[p.name] = 0
    else params[p.name] = ''
  }
  const newNode: BtNodeInternal = { type: meta.type_name, category: meta.category, params }
  if (meta.category === 'composite') newNode.children = []
  if (meta.category === 'decorator') newNode.child = null
  return newNode
}

function onRootTypeSelected(meta: BtNodeTypeMeta) {
  rootNode.value = makeDefaultNode(meta)
}

// ─── name check ───

async function checkNameUnique() {
  if (!form.name || !namePattern.test(form.name)) {
    nameStatus.value = ''
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await btTreeApi.checkName(form.name)
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

// ─── submit ───

async function handleSubmit() {
  if (!rootNode.value) {
    treeError.value = '请设置根节点'
    return
  }
  treeError.value = ''

  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.warning('标识符已被使用，请更换')
    return
  }

  submitting.value = true
  try {
    const config = serializeBtNode(rootNode.value)

    if (isCreate) {
      await btTreeApi.create({
        name: form.name,
        display_name: form.display_name,
        description: form.description,
        config,
      })
      ElMessage.success('创建成功，行为树默认为禁用状态，确认无误后请手动启用')
    } else {
      await btTreeApi.update({
        id: Number(route.params.id),
        version: version.value,
        display_name: form.display_name,
        description: form.description,
        config,
      })
      ElMessage.success('保存成功')
    }
    router.push('/bt-trees')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === BT_TREE_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他人修改，请刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (bizErr.code === BT_TREE_ERR.NAME_EXISTS || bizErr.code === BT_TREE_ERR.NAME_INVALID) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === BT_TREE_ERR.NOT_FOUND) {
      ElMessage.error('行为树不存在')
      router.push('/bt-trees')
      return
    }
    if (bizErr.code === BT_TREE_ERR.EDIT_NOT_DISABLED) {
      ElMessage.warning('请先禁用该行为树后再编辑')
      return
    }
    if (
      bizErr.code === BT_TREE_ERR.CONFIG_INVALID ||
      bizErr.code === BT_TREE_ERR.NODE_TYPE_NOT_FOUND ||
      bizErr.code === BT_TREE_ERR.DEPTH_EXCEEDED
    ) {
      treeError.value = bizErr.message
      return
    }
    // Other errors: global interceptor handles toast
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.bt-tree-form {
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

.loading-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #909399;
  padding: 12px 0;
}

.no-root-area {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
}

.no-root-hint {
  font-size: 13px;
  color: #909399;
}

.tree-error {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  font-size: 13px;
  color: #F56C6C;
}
</style>
