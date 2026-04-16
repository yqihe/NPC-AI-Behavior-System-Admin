<template>
  <div class="bt-node-type-list">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">节点类型管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/bt-node-types/create')">
          <el-icon><Plus /></el-icon>
          新建节点类型
        </el-button>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-input
        v-model="query.type_name"
        placeholder="搜索节点类型标识"
        clearable
        class="filter-item filter-item-wide"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.category"
        placeholder="节点分类"
        clearable
        class="filter-item"
      >
        <el-option label="组合节点" value="composite" />
        <el-option label="装饰节点" value="decorator" />
        <el-option label="叶子节点" value="leaf" />
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
        <el-table-column prop="type_name" label="节点标识" min-width="160" />
        <el-table-column prop="label" label="中文名" min-width="120" />
        <el-table-column label="分类" width="110">
          <template #default="{ row }">
            <el-tag
              :type="categoryTagType(row.category)"
              size="small"
            >
              {{ categoryLabel(row.category) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="内置" width="90">
          <template #default="{ row }">
            <el-tag
              :type="row.is_builtin ? 'info' : ''"
              size="small"
            >
              {{ row.is_builtin ? '内置' : '自定义' }}
            </el-tag>
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
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click="router.push(`/bt-node-types/${row.id}/view`)">查看</el-link>
            <template v-if="!row.is_builtin">
              <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
              <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
            </template>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无节点类型数据">
            <el-button type="primary" @click="$router.push('/bt-node-types/create')">
              新建节点类型
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

    <!-- 引用弹窗 -->
    <el-dialog
      v-model="refDialog.visible"
      :title="`引用详情 — ${refDialog.label} (${refDialog.typeName})`"
      width="480px"
      @close="resetRefDialog"
    >
      <p class="ref-subtitle">以下行为树使用了该节点类型，无法删除。请先在行为树编辑器中移除相关节点：</p>
      <el-table :data="refDialog.referencedBy.map(n => ({ name: n }))" size="small">
        <el-table-column prop="name" label="行为树标识" />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import { btNodeTypeApi, BT_NODE_TYPE_ERR } from '@/api/btNodeTypes'
import type { BtNodeTypeListItem, BtNodeTypeListQuery } from '@/api/btNodeTypes'
import type { BizError } from '@/api/request'

const router = useRouter()

const loading = ref(false)
const tableData = ref<BtNodeTypeListItem[]>([])
const total = ref(0)
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const query = reactive<BtNodeTypeListQuery>({
  type_name: '',
  category: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

const refDialog = reactive({
  visible: false,
  typeName: '',
  label: '',
  referencedBy: [] as string[],
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: BtNodeTypeListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.type_name) params.type_name = query.type_name
    if (query.category) params.category = query.category
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await btNodeTypeApi.list(params)
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
  query.type_name = ''
  query.category = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: BtNodeTypeListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用节点类型「${row.label}」？启用后树编辑器可使用该节点类型。`
    : `确认禁用节点类型「${row.label}」？禁用后树编辑器将不再展示该节点类型。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    const detail = await btNodeTypeApi.detail(row.id)
    await btNodeTypeApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === BT_NODE_TYPE_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他人修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: BtNodeTypeListItem) {
  if (row.is_builtin) {
    ElMessage.warning('内置节点类型不可编辑')
    return
  }
  if (row.enabled) {
    guardRef.value?.open({
      action: 'edit',
      entityType: 'bt-node-type',
      entity: { id: row.id, name: row.type_name, label: row.label },
    })
    return
  }
  router.push(`/bt-node-types/${row.id}/edit`)
}

async function handleDelete(row: BtNodeTypeListItem) {
  if (row.is_builtin) {
    ElMessage.warning('内置节点类型不可删除')
    return
  }
  if (row.enabled) {
    guardRef.value?.open({
      action: 'delete',
      entityType: 'bt-node-type',
      entity: { id: row.id, name: row.type_name, label: row.label },
    })
    return
  }
  try {
    await btNodeTypeApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === BT_NODE_TYPE_ERR.REF_DELETE) {
      refDialog.typeName = row.type_name
      refDialog.label = row.label
      refDialog.referencedBy = (bizErr.data as { referenced_by: string[] })?.referenced_by ?? []
      refDialog.visible = true
      return
    }
    // 其他错误拦截器已 toast
  }
}

function resetRefDialog() {
  refDialog.typeName = ''
  refDialog.label = ''
  refDialog.referencedBy = []
}

// ---------- 辅助 ----------

function categoryTagType(category: string): '' | 'warning' | 'success' {
  if (category === 'composite') return ''
  if (category === 'decorator') return 'warning'
  if (category === 'leaf') return 'success'
  return ''
}

function categoryLabel(category: string): string {
  if (category === 'composite') return '组合节点'
  if (category === 'decorator') return '装饰节点'
  if (category === 'leaf') return '叶子节点'
  return category
}

function rowClassName({ row }: { row: BtNodeTypeListItem }) {
  return row.enabled ? '' : 'row-disabled'
}
</script>

<style scoped>
.bt-node-type-list {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

:deep(.row-disabled td:not(:nth-last-child(-n+3))) {
  opacity: 0.5;
}

.ref-subtitle {
  font-size: 13px;
  color: #606266;
  margin: 0 0 12px;
  line-height: 1.6;
}
</style>
