<template>
  <div class="event-type-list">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">事件类型管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/event-types/create')">
          <el-icon><Plus /></el-icon>
          新建事件类型
        </el-button>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-input
        v-model="query.label"
        placeholder="搜索中文标签"
        clearable
        class="filter-item filter-item-wide"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.perception_mode"
        placeholder="感知模式"
        clearable
        class="filter-item"
      >
        <el-option label="视觉 (Visual)" value="visual" />
        <el-option label="听觉 (Auditory)" value="auditory" />
        <el-option label="全局 (Global)" value="global" />
      </el-select>
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
        <el-table-column prop="name" label="事件标识" min-width="140" />
        <el-table-column prop="display_name" label="中文标签" min-width="120" />
        <el-table-column label="感知模式" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="modeBadgeType(row.perception_mode)">
              {{ modeLabel(row.perception_mode) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="严重度" width="80" align="center">
          <template #default="{ row }">{{ row.default_severity }}</template>
        </el-table-column>
        <el-table-column label="TTL" width="80" align="center">
          <template #default="{ row }">{{ row.default_ttl }}</template>
        </el-table-column>
        <el-table-column label="范围" width="80" align="center">
          <template #default="{ row }">{{ row.range }}</template>
        </el-table-column>
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
            <el-link type="primary" :underline="false" @click="$router.push(`/event-types/${row.id}/view`)">
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
          <el-empty description="暂无事件类型数据">
            <el-button type="primary" @click="$router.push('/event-types/create')">
              新建事件类型
            </el-button>
          </el-empty>
        </template>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrap">
        <span class="total-text">共 {{ total }} 条</span>
        <el-pagination
          v-model:current-page="query.page"
          :page-size="query.page_size"
          :total="total"
          layout="prev, pager, next"
          @current-change="fetchList"
        />
      </div>
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
import { eventTypeApi, EVENT_TYPE_ERR } from '@/api/eventTypes'
import type { EventTypeListItem, EventTypeListQuery } from '@/api/eventTypes'
import type { BizError } from '@/api/request'

const router = useRouter()

const loading = ref(false)
const tableData = ref<EventTypeListItem[]>([])
const total = ref(0)
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const query = reactive<EventTypeListQuery>({
  label: '',
  perception_mode: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: EventTypeListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.label) params.label = query.label
    if (query.perception_mode) params.perception_mode = query.perception_mode
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await eventTypeApi.list(params)
    tableData.value = res.data?.items || []
    total.value = res.data?.total || 0
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
  query.page = 1
  fetchList()
}

function handleReset() {
  query.label = ''
  query.perception_mode = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: EventTypeListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用事件类型「${row.display_name}」？启用后可被 FSM/BT 引用。`
    : `确认禁用事件类型「${row.display_name}」？禁用后新的 FSM/BT 无法引用该事件类型，已有引用不受影响。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    const detail = await eventTypeApi.detail(row.id)
    await eventTypeApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === EVENT_TYPE_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: EventTypeListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'edit',
      entityType: 'event-type',  // T5 扩展后移除 as cast
      entity: { id: row.id, name: row.name, label: row.display_name, ref_count: 0 },
    })
    return
  }
  router.push(`/event-types/${row.id}/edit`)
}

async function handleDelete(row: EventTypeListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'delete',
      entityType: 'event-type',  // T5 扩展后移除 as cast
      entity: { id: row.id, name: row.name, label: row.display_name, ref_count: 0 },
    })
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除事件类型「${row.display_name}」（${row.name}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await eventTypeApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    // 其他错误拦截器已 toast
  }
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: EventTypeListItem }) {
  return row.enabled ? '' : 'row-disabled'
}

function modeBadgeType(mode: string): '' | 'success' | 'info' | 'warning' | 'danger' {
  const map: Record<string, '' | 'success' | 'info' | 'warning' | 'danger'> = {
    visual: 'success',
    auditory: '',
    global: 'info',
  }
  return map[mode] || 'info'
}

function modeLabel(mode: string): string {
  const map: Record<string, string> = {
    visual: '视觉',
    auditory: '听觉',
    global: '全局',
  }
  return map[mode] || mode
}

function formatTime(str: string) {
  if (!str) return ''
  const d = new Date(str)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.event-type-list {
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
  flex: 1;
  min-width: 0;
}

.filter-item-wide {
  flex: 1.5;
}

.table-wrap {
  flex: 1;
  padding: 0 24px;
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.pagination-wrap {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
}

.total-text {
  font-size: 13px;
  color: #909399;
}

:deep(.row-disabled td:not(:nth-last-child(-n+3))) {
  opacity: 0.5;
}
</style>
