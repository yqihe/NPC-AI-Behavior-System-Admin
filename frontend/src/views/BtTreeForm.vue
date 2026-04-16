<template>
  <div class="bt-tree-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="router.back()"><ArrowLeft /></el-icon>
      <span class="back-text" @click="router.back()">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ isView ? '查看行为树' : isCreate ? '新建行为树' : '编辑行为树' }}
      </span>
    </div>

    <!-- 表单滚动区 -->
    <div class="form-scroll">
      <div class="form-body-wide">

        <!-- Card 1: 基本信息 -->
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
            <!-- 行为树标识 -->
            <el-form-item label="行为树标识" prop="name">
              <template v-if="!isCreate || isView">
                <el-input :model-value="form.name" disabled style="width: 100%">
                  <template #prefix>
                    <el-icon><Lock /></el-icon>
                  </template>
                </el-input>
                <div class="field-warn">
                  <el-icon><WarningFilled /></el-icon>
                  创建后不可修改
                </div>
              </template>
              <template v-else>
                <el-input
                  v-model="form.name"
                  placeholder="如 npc/patrol（小写字母开头，仅含小写字母、数字、下划线、斜线）"
                  style="width: 100%"
                  @blur="checkNameUnique"
                />
                <div v-if="nameStatus === 'checking'" class="field-hint">
                  <el-icon class="is-loading"><Loading /></el-icon>
                  校验中...
                </div>
                <div v-else-if="nameStatus === 'available'" class="field-hint field-hint-success">
                  <el-icon><CircleCheck /></el-icon>
                  标识符可用
                </div>
                <div v-else-if="nameStatus === 'taken'" class="field-hint field-hint-error">
                  <el-icon><CircleClose /></el-icon>
                  {{ nameMessage }}
                </div>
                <div class="field-hint field-gray">
                  格式：小写字母、数字、下划线、斜线，以字母开头，如 npc/patrol
                </div>
              </template>
            </el-form-item>

            <!-- 中文名称 -->
            <el-form-item label="中文名称" prop="display_name">
              <el-input
                v-model="form.display_name"
                placeholder="如 巡逻行为树"
                style="width: 100%"
              />
            </el-form-item>

            <!-- 描述 -->
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

        <!-- Card 2: 行为树结构 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-green"></span>
            <span class="title-text">行为树结构</span>
          </div>

          <div v-if="loadingNodeTypes" class="loading-hint">
            <el-icon class="is-loading"><Loading /></el-icon>
            加载节点类型...
          </div>

          <template v-else>
            <BtNodeEditor
              :model-value="rootNode"
              :node-types="nodeTypeMetas"
              :disabled="isView"
              :depth="0"
              @update:model-value="rootNode = $event"
            />

            <div v-if="treeError" class="tree-error">
              <el-icon><WarningFilled /></el-icon>
              {{ treeError }}
            </div>
          </template>
        </div>

      </div>
    </div>

    <!-- 底部操作栏（查看模式隐藏） -->
    <div v-if="!isView" class="form-footer">
      <el-button @click="router.back()">取消</el-button>
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
import BtNodeEditor from '@/components/BtNodeEditor.vue'
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

const loadingNodeTypes = ref(false)
const nodeTypeMetas = ref<BtNodeTypeMeta[]>([])

const namePattern = /^[a-z][a-z0-9_/]*$/

const rules = {
  name: [
    { required: true, message: '请输入行为树标识', trigger: 'blur' },
    {
      pattern: namePattern,
      message: '小写字母开头，仅含小写字母、数字、下划线、斜线',
      trigger: 'blur',
    },
  ],
  display_name: [
    { required: true, message: '请输入中文名称', trigger: 'blur' },
  ],
}

// ─── 初始化 ───

onMounted(async () => {
  await loadNodeTypes()
  if (!isCreate) await loadDetail()
})

async function loadNodeTypes() {
  loadingNodeTypes.value = true
  try {
    const listRes = await btNodeTypeApi.list({ enabled: true, page: 1, page_size: 200 })
    const items = listRes.data?.items ?? []

    // 并行拉取详情以获取 param_schema（列表接口不含此字段）
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
    // 拦截器已 toast
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

// ─── 标识符校验 ───

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

// ─── 提交 ───

async function handleSubmit() {
  if (!rootNode.value) {
    ElMessage.warning('请先构建行为树结构')
    treeError.value = '请先构建行为树结构'
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
    // 其他错误拦截器已 toast
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
/* 仅组件私有样式；form-header/form-scroll/form-body-wide/form-card/card-title/
   title-bar/title-text/form-footer 均由全局 form-layout.css 提供，此处不重复 */

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
.field-gray         { color: #909399; }

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

.tree-error {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  font-size: 13px;
  color: #F56C6C;
}
</style>
