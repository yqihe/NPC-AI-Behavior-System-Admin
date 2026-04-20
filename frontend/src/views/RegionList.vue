<template>
  <div class="list-root">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">区域管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/regions/create')">
          <el-icon><Plus /></el-icon>
          新建区域
        </el-button>
      </div>
    </div>

    <div class="filter-bar">
      <el-input
        v-model="query.region_id"
        placeholder="搜索区域标识"
        clearable
        class="filter-item"
        @keyup.enter="handleSearch"
      />
      <el-input
        v-model="query.display_name"
        placeholder="搜索中文名"
        clearable
        class="filter-item"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.region_type"
        placeholder="区域类型"
        clearable
        class="filter-item"
      >
        <el-option
          v-for="opt in regionTypeOptions"
          :key="opt.name"
          :label="opt.label"
          :value="opt.name"
        />
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

    <div class="table-wrap">
      <el-table
        v-loading="loading"
        :data="tableData"
        :row-class-name="rowClassName"
        style="width: 100%"
      >
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="region_id" label="区域标识" min-width="160" />
        <el-table-column prop="display_name" label="中文名" min-width="140" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            {{ regionTypeLabel(row.region_type) }}
          </template>
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
            <el-link type="primary" :underline="false" @click="router.push(`/regions/${row.id}/view`)">查看</el-link>
            <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
            <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无区域数据">
            <el-button type="primary" @click="$router.push('/regions/create')">
              新建区域
            </el-button>
          </el-empty>
        </template>
      </el-table>

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

    <EnabledGuardDialog ref="guardRef" @refresh="fetchList" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import { regionApi, REGION_ERR } from '@/api/regions'
import type { RegionListItem, RegionListQuery } from '@/api/regions'
import type { DictionaryItem } from '@/api/dictionaries'
import type { BizError } from '@/api/request'
import { formatTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const tableData = ref<RegionListItem[]>([])
const total = ref(0)
const regionTypeOptions = ref<DictionaryItem[]>([])
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const query = reactive<RegionListQuery>({
  region_id: '',
  display_name: '',
  region_type: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 区域类型字典 ----------

async function loadRegionTypeOptions() {
  try {
    const res = await regionApi.getRegionTypeOptions()
    regionTypeOptions.value = res.data?.items || []
  } catch {
    // 拦截器已 toast
  }
}

function regionTypeLabel(name: string): string {
  const hit = regionTypeOptions.value.find((o) => o.name === name)
  return hit ? hit.label : name
}

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: RegionListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.region_id) params.region_id = query.region_id
    if (query.display_name) params.display_name = query.display_name
    if (query.region_type) params.region_type = query.region_type
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await regionApi.list(params)
    tableData.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadRegionTypeOptions()
  fetchList()
})

// ---------- 筛选 ----------

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.region_id = ''
  query.display_name = ''
  query.region_type = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: RegionListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用区域「${row.display_name}」？启用后游戏服务端可拉取该区域配置。`
    : `确认禁用区域「${row.display_name}」？禁用后游戏服务端将不再拉取该区域配置。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    const detail = await regionApi.detail(row.id)
    await regionApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === REGION_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他人修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: RegionListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'edit',
      entityType: 'region',
      entity: { id: row.id, name: row.region_id, label: row.display_name },
    })
    return
  }
  router.push(`/regions/${row.id}/edit`)
}

async function handleDelete(row: RegionListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'delete',
      entityType: 'region',
      entity: { id: row.id, name: row.region_id, label: row.display_name },
    })
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除区域「${row.display_name}」（${row.region_id}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await regionApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    const code = (err as BizError).code
    if (code === REGION_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast（47008 由拦截器以中文提示）
  }
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: RegionListItem }) {
  return row.enabled ? '' : 'row-disabled'
}
</script>
