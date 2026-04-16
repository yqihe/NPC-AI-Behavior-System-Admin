<template>
  <div class="fsm-config-list">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">状态机管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/fsm-configs/create')">
          <el-icon><Plus /></el-icon>
          新建状态机
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
        <el-table-column prop="name" label="状态机标识" min-width="140" />
        <el-table-column prop="display_name" label="中文标签" min-width="120" />
        <el-table-column label="初始状态" width="140">
          <template #default="{ row }">{{ row.initial_state_label || row.initial_state }}</template>
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
            <el-link type="primary" :underline="false" @click="router.push(`/fsm-configs/${row.id}/view`)">查看</el-link>
            <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
            <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无状态机数据">
            <el-button type="primary" @click="$router.push('/fsm-configs/create')">
              新建状态机
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
import { fsmConfigApi, FSM_ERR } from '@/api/fsmConfigs'
import type { FsmConfigListItem, FsmConfigListQuery } from '@/api/fsmConfigs'
import type { BizError } from '@/api/request'
import { formatTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const tableData = ref<FsmConfigListItem[]>([])
const total = ref(0)
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const query = reactive<FsmConfigListQuery>({
  label: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: FsmConfigListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.label) params.label = query.label
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await fsmConfigApi.list(params)
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
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: FsmConfigListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用状态机「${row.display_name}」？启用后游戏服务端可拉取该状态机。`
    : `确认禁用状态机「${row.display_name}」？禁用后游戏服务端将不再拉取该状态机。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    const detail = await fsmConfigApi.detail(row.id)
    await fsmConfigApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === FSM_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他人修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: FsmConfigListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'edit',
      entityType: 'fsm-config',
      entity: { id: row.id, name: row.name, label: row.display_name },
    })
    return
  }
  router.push(`/fsm-configs/${row.id}/edit`)
}

async function handleDelete(row: FsmConfigListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'delete',
      entityType: 'fsm-config',
      entity: { id: row.id, name: row.name, label: row.display_name },
    })
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除状态机「${row.display_name}」（${row.name}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await fsmConfigApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    if ((err as BizError).code === FSM_ERR.VERSION_CONFLICT) {
      ElMessage.warning('数据已更新，请重新操作')
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: FsmConfigListItem }) {
  return row.enabled ? '' : 'row-disabled'
}
</script>

<style scoped>
.fsm-config-list {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

:deep(.row-disabled td:not(:nth-last-child(-n+3))) {
  opacity: 0.5;
}
</style>
