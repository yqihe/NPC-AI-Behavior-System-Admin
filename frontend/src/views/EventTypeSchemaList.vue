<template>
  <div class="schema-list">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">事件字段管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/event-type-schemas/create')">
          <el-icon><Plus /></el-icon>
          新建扩展字段
        </el-button>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-select
        v-model="query.enabled"
        placeholder="启用状态"
        clearable
        class="filter-item"
      >
        <el-option label="已启用" :value="true" />
        <el-option label="已禁用" :value="false" />
      </el-select>
      <el-button type="primary" @click="handleSearch">
        <el-icon><Search /></el-icon>
        搜索
      </el-button>
      <el-button @click="handleReset">重置</el-button>
      <div class="filter-spacer"></div>
      <el-button-group class="sort-toggle">
        <el-button
          :type="sortMode === 'id_desc' ? 'primary' : 'default'"
          size="small"
          @click="setSortMode('id_desc')"
        >
          ID 倒序
        </el-button>
        <el-button
          :type="sortMode === 'sort_asc' ? 'primary' : 'default'"
          size="small"
          @click="setSortMode('sort_asc')"
        >
          排序正序
        </el-button>
      </el-button-group>
    </div>

    <!-- 数据表格 -->
    <div class="table-wrap">
      <el-table
        v-loading="loading"
        :data="tableData"
        :row-class-name="rowClassName"
        style="width: 100%"
      >
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="field_name" label="字段标识" min-width="130" />
        <el-table-column prop="field_label" label="中文标签" min-width="120" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="typeBadgeType(row.field_type)">
              {{ typeLabel(row.field_type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sort_order" label="排序" width="80" align="center" />
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enabled"
              @change="(val: string | number | boolean) => handleToggle(row, Boolean(val))"
            />
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click="router.push(`/event-type-schemas/${row.id}/view`)">查看</el-link>
            <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
            <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无扩展字段定义">
            <el-button type="primary" @click="$router.push('/event-type-schemas/create')">
              新建扩展字段
            </el-button>
          </el-empty>
        </template>
      </el-table>
    </div>

    <!-- 启用守卫弹窗 -->
    <EnabledGuardDialog ref="guardRef" @refresh="fetchList" />

    <!-- 引用详情弹窗 -->
    <el-dialog
      v-model="refDialog.visible"
      :title="`引用详情 — ${refDialog.fieldLabel}`"
      width="500px"
      @close="resetRefDialog"
    >
      <div v-loading="refDialog.loading">
        <div class="ref-section">
          <p class="ref-subtitle">
            事件类型引用（{{ refDialog.eventTypes.length }} 个事件类型使用了该扩展字段）：
          </p>
          <el-table
            v-if="refDialog.eventTypes.length > 0"
            :data="refDialog.eventTypes"
            size="small"
          >
            <el-table-column prop="label" label="事件类型名称" />
            <el-table-column prop="ref_type" label="类型" width="120" />
          </el-table>
          <p v-else class="ref-empty">暂无事件类型引用</p>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import { eventTypeApi, EXT_SCHEMA_ERR } from '@/api/eventTypes'
import type { EventTypeSchemaFull, ExtSchemaListQuery, SchemaReferenceItem } from '@/api/eventTypes'
import type { BizError } from '@/api/request'
import { formatTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const tableData = ref<EventTypeSchemaFull[]>([])
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

type SortMode = 'id_desc' | 'sort_asc'
const sortMode = ref<SortMode>('id_desc')

const query = reactive<ExtSchemaListQuery>({
  enabled: undefined,
})

const refDialog = reactive({
  visible: false,
  loading: false,
  fieldLabel: '',
  eventTypes: [] as SchemaReferenceItem[],
})

function resetRefDialog() {
  refDialog.loading = false
  refDialog.fieldLabel = ''
  refDialog.eventTypes = []
}

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: ExtSchemaListQuery = {}
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await eventTypeApi.schemaList(params)
    const items = res.data?.items || []
    applySorting(items)
    tableData.value = items
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchList()
})

// ---------- 筛选 ----------

function handleSearch() {
  fetchList()
}

function handleReset() {
  query.enabled = undefined
  fetchList()
}

function applySorting(items: EventTypeSchemaFull[]) {
  if (sortMode.value === 'sort_asc') {
    items.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
  } else {
    items.sort((a, b) => b.id - a.id)
  }
}

function setSortMode(mode: SortMode) {
  if (sortMode.value === mode) return
  sortMode.value = mode
  applySorting(tableData.value)
}

// ---------- 行操作 ----------

async function handleToggle(row: EventTypeSchemaFull, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用扩展字段「${row.field_label}」？启用后将在事件类型表单中可用。`
    : `确认禁用扩展字段「${row.field_label}」？禁用后新事件类型无法使用该字段，已有配置不受影响。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    // 预取最新 version（无 detail API，走 list 刷新）
    const freshRes = await eventTypeApi.schemaList()
    const freshRow = (freshRes.data?.items || []).find((s) => s.id === row.id)
    if (!freshRow) {
      ElMessage.error('扩展字段不存在，可能已被删除')
      fetchList()
      return
    }
    await eventTypeApi.schemaToggleEnabled(row.id, val, freshRow.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === EXT_SCHEMA_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: EventTypeSchemaFull) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'edit',
      entityType: 'event-type-schema',
      entity: { id: row.id, name: row.field_name, label: row.field_label },
    })
    return
  }
  router.push(`/event-type-schemas/${row.id}/edit`)
}

async function handleDelete(row: EventTypeSchemaFull) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'delete',
      entityType: 'event-type-schema',
      entity: { id: row.id, name: row.field_name, label: row.field_label },
    })
    return
  }
  // 已禁用：先查引用，有引用弹详情阻止，无引用确认删除
  try {
    const res = await eventTypeApi.schemaReferences(row.id)
    const ets = res.data?.event_types || []
    if (ets.length > 0) {
      showRefDialog(row, ets)
      ElMessage.warning(`该扩展字段被 ${ets.length} 个事件类型使用，无法删除。请先移除引用关系。`)
      return
    }
  } catch {
    // references 失败拦截器已 toast；为安全起见不继续删除
    return
  }
  // 无引用：确认删除
  try {
    await ElMessageBox.confirm(
      `确认删除扩展字段「${row.field_label}」（${row.field_name}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await eventTypeApi.schemaDelete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    const code = (err as BizError).code
    if (code === EXT_SCHEMA_ERR.DELETE_NOT_DISABLED) {
      ElMessage.warning('请先禁用该扩展字段后再删除')
      return
    }
    if (code === EXT_SCHEMA_ERR.REF_DELETE) {
      // 后端兜底：重新拉引用详情展示
      await loadAndShowRefs(row)
      return
    }
    // 其他错误拦截器已 toast
  }
}

function showRefDialog(row: EventTypeSchemaFull, eventTypes: SchemaReferenceItem[]) {
  refDialog.visible = true
  refDialog.loading = false
  refDialog.fieldLabel = row.field_label
  refDialog.eventTypes = eventTypes
}

async function loadAndShowRefs(row: EventTypeSchemaFull) {
  refDialog.visible = true
  refDialog.loading = true
  refDialog.fieldLabel = row.field_label
  refDialog.eventTypes = []
  try {
    const res = await eventTypeApi.schemaReferences(row.id)
    refDialog.eventTypes = res.data?.event_types || []
  } catch {
    // 拦截器已 toast
  } finally {
    refDialog.loading = false
  }
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: EventTypeSchemaFull }) {
  return row.enabled ? '' : 'row-disabled'
}

function typeBadgeType(type: string) {
  const map: Record<string, string> = {
    int: '',
    float: '',
    string: 'success',
    bool: 'warning',
    select: 'info',
  }
  return map[type] || 'info'
}

function typeLabel(type: string) {
  const map: Record<string, string> = {
    int: '整数',
    float: '浮点数',
    string: '文本',
    bool: '布尔',
    select: '选择',
  }
  return map[type] || type
}

</script>

<style scoped>
.schema-list {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.filter-spacer {
  flex: 1;
}

.sort-toggle {
  flex-shrink: 0;
}

:deep(.row-disabled td:not(:nth-last-child(-n+3))) {
  opacity: 0.5;
}

.ref-section {
  margin-bottom: 8px;
}

.ref-subtitle {
  font-size: 13px;
  color: #909399;
  margin: 0 0 8px 0;
}

.ref-empty {
  font-size: 13px;
  color: #C0C4CC;
  margin: 4px 0;
}
</style>
