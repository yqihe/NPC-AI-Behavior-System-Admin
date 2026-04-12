<template>
  <div class="schema-list">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">扩展字段管理</h2>
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
            <el-link type="primary" :underline="false" @click="$router.push(`/event-type-schemas/${row.id}/view`)">
              查看
            </el-link>
            <el-link type="primary" :underline="false" @click="handleEdit(row)" style="margin-left: 12px">
              编辑
            </el-link>
            <el-link type="danger" :underline="false" @click="handleDelete(row)" style="margin-left: 12px">
              删除
            </el-link>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import { eventTypeApi, EXT_SCHEMA_ERR } from '@/api/eventTypes'
import type { EventTypeSchemaFull, ExtSchemaListQuery } from '@/api/eventTypes'
import type { BizError } from '@/api/request'

const router = useRouter()

const loading = ref(false)
const tableData = ref<EventTypeSchemaFull[]>([])
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const query = reactive<ExtSchemaListQuery>({
  enabled: undefined,
})

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
    // 后端按 sort_order ASC 排序，前端统一按 ID 倒序展示（与其他列表一致）
    items.sort((a, b) => b.id - a.id)
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
    await eventTypeApi.schemaToggleEnabled(row.id, val, row.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === EXT_SCHEMA_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: EventTypeSchemaFull) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'edit',
      entityType: 'event-type-schema',
      entity: { id: row.id, name: row.field_name, label: row.field_label, ref_count: 0 },
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
      entity: { id: row.id, name: row.field_name, label: row.field_label, ref_count: 0 },
    })
    return
  }
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
    if ((err as BizError).code === EXT_SCHEMA_ERR.DELETE_NOT_DISABLED) {
      ElMessage.warning('请先禁用该扩展字段后再删除')
    }
    // 其他错误拦截器已 toast
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

function formatTime(str: string) {
  if (!str) return ''
  const d = new Date(str)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.schema-list {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background: #fff;
  border-bottom: 1px solid #E4E7ED;
}

.page-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
  background: #fff;
  flex-wrap: wrap;
}

.filter-item {
  width: 180px;
}

.table-wrap {
  flex: 1;
  padding: 0 24px;
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

:deep(.row-disabled td:not(:nth-last-child(-n+3))) {
  opacity: 0.5;
}
</style>
