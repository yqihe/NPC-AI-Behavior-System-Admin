<template>
  <div class="fsm-state-dict-list">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">状态字典管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/fsm-state-dicts/create')">
          <el-icon><Plus /></el-icon>
          新建状态
        </el-button>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-input
        v-model="query.name"
        placeholder="搜索状态标识"
        clearable
        class="filter-item filter-item-wide"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.category"
        placeholder="分类"
        clearable
        class="filter-item"
      >
        <el-option
          v-for="cat in categories"
          :key="cat"
          :label="cat"
          :value="cat"
        />
      </el-select>
      <el-select
        v-model="query.enabled"
        placeholder="启用状态"
        clearable
        class="filter-item"
      >
        <el-option label="已启用" :value="true" />
        <el-option label="已停用" :value="false" />
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
        <el-table-column prop="name" label="状态标识" min-width="160" />
        <el-table-column prop="display_name" label="中文名" min-width="140" />
        <el-table-column prop="category" label="分类" width="120" />
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
            <el-link type="primary" :underline="false" @click="$router.push(`/fsm-state-dicts/${row.id}/view`)">
              查看
            </el-link>
            <el-link type="primary" :underline="false" @click="$router.push(`/fsm-state-dicts/${row.id}/edit`)" style="margin-left: 12px">
              编辑
            </el-link>
            <el-link type="danger" :underline="false" @click="handleDelete(row)" style="margin-left: 12px">
              删除
            </el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无状态字典数据">
            <el-button type="primary" @click="$router.push('/fsm-state-dicts/create')">
              新建状态
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

    <!-- 43020 被 FSM 引用弹窗 -->
    <el-dialog
      v-model="refDeleteVisible"
      :title="`无法删除「${refDeleteResult?.display_name ?? ''}」`"
      width="540px"
      :close-on-click-modal="false"
      append-to-body
    >
      <p class="ref-delete-lead">以下 FSM 配置引用了此状态，请先修改再删除：</p>
      <el-table :data="refDeleteResult?.referenced_by ?? []" style="width: 100%" size="small">
        <el-table-column prop="name" label="配置标识" min-width="140" />
        <el-table-column prop="display_name" label="中文名" min-width="140" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="refDeleteVisible = false">知道了</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import { fsmStateDictApi, FSM_STATE_DICT_ERR } from '@/api/fsmStateDicts'
import type {
  FsmStateDictListItem,
  FsmStateDictListQuery,
  FsmStateDictDeleteResult,
} from '@/api/fsmStateDicts'
import type { BizError } from '@/api/request'

const loading = ref(false)
const tableData = ref<FsmStateDictListItem[]>([])
const total = ref(0)
const categories = ref<string[]>([])
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const refDeleteVisible = ref(false)
const refDeleteResult = ref<FsmStateDictDeleteResult | null>(null)

const query = reactive<FsmStateDictListQuery>({
  name: '',
  category: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: FsmStateDictListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.name) params.name = query.name
    if (query.category) params.category = query.category
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await fsmStateDictApi.list(params)
    tableData.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

async function fetchCategories() {
  try {
    const res = await fsmStateDictApi.listCategories()
    categories.value = res.data ?? []
  } catch {
    // 非关键，静默失败
  }
}

onMounted(() => {
  fetchList()
  fetchCategories()
})

// ---------- 筛选 ----------

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.name = ''
  query.category = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: FsmStateDictListItem, val: boolean) {
  const action = val ? '启用' : '停用'
  const msg = val
    ? `确认启用状态「${row.display_name}」（${row.name}）？`
    : `确认停用状态「${row.display_name}」（${row.name}）？停用后仍可被已有 FSM 配置引用，但新引用需重新启用。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    const detail = await fsmStateDictApi.detail(row.id)
    await fsmStateDictApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') { fetchList(); return }
    if ((err as BizError).code === FSM_STATE_DICT_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

async function handleDelete(row: FsmStateDictListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'delete',
      entityType: 'fsm-state-dict',
      entity: { id: row.id, name: row.name, label: row.display_name },
    })
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除状态「${row.display_name}」（${row.name}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await fsmStateDictApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    const bizErr = err as BizError
    if (bizErr.code === FSM_STATE_DICT_ERR.IN_USE) {
      refDeleteResult.value = bizErr.data as FsmStateDictDeleteResult
      refDeleteVisible.value = true
      return
    }
    // 其他错误拦截器已 toast
  }
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: FsmStateDictListItem }) {
  return row.enabled ? '' : 'row-disabled'
}

function formatTime(str: string) {
  if (!str) return ''
  const d = new Date(str)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.fsm-state-dict-list {
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

.ref-delete-lead {
  font-size: 14px;
  color: #606266;
  margin: 0 0 12px;
}

:deep(.row-disabled td:not(:nth-last-child(-n+3))) {
  opacity: 0.5;
}
</style>
