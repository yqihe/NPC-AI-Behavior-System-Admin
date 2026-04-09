<template>
  <div class="field-list">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">字段管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/fields/create')">
          <el-icon><Plus /></el-icon>
          新建字段
        </el-button>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-input
        v-model="query.label"
        placeholder="搜索中文标签"
        clearable
        style="width: 200px"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.type"
        placeholder="字段类型"
        clearable
        style="width: 160px"
      >
        <el-option
          v-for="item in typeOptions"
          :key="item.name"
          :label="item.label"
          :value="item.name"
        />
      </el-select>
      <el-select
        v-model="query.category"
        placeholder="字段分类"
        clearable
        style="width: 160px"
      >
        <el-option
          v-for="item in categoryOptions"
          :key="item.name"
          :label="item.label"
          :value="item.name"
        />
      </el-select>
      <el-select
        v-model="query.enabled"
        placeholder="状态"
        clearable
        style="width: 140px"
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
        <el-table-column prop="name" label="标识符" min-width="120" />
        <el-table-column prop="label" label="中文标签" min-width="120" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="typeBadgeType(row.type)">
              {{ row.type_label || row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="分类" width="100">
          <template #default="{ row }">
            <el-tag size="small" type="info">
              {{ row.category_label || row.category }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="引用数" width="80" align="center">
          <template #default="{ row }">
            <el-link
              v-if="row.ref_count > 0"
              type="primary"
              :underline="false"
              @click="handleShowRefs(row)"
            >
              {{ row.ref_count }}
            </el-link>
            <span v-else class="ref-zero">0</span>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enabled"
              @change="(val) => handleToggle(row, val)"
            />
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click="handleEdit(row)">
              编辑
            </el-link>
            <el-link type="danger" :underline="false" @click="handleDelete(row)" style="margin-left: 12px">
              删除
            </el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无字段数据">
            <el-button type="primary" @click="$router.push('/fields/create')">
              新建字段
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
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import { fieldApi } from '@/api/fields'
import { dictApi } from '@/api/dictionaries'

const router = useRouter()

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const typeOptions = ref([])
const categoryOptions = ref([])

const query = reactive({
  label: '',
  type: '',
  category: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.label) params.label = query.label
    if (query.type) params.type = query.type
    if (query.category) params.category = query.category
    if (query.enabled !== null && query.enabled !== '') {
      params.enabled = query.enabled
    }
    const res = await fieldApi.list(params)
    tableData.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

async function loadDictionaries() {
  try {
    const [typeRes, catRes] = await Promise.all([
      dictApi.list('field_type'),
      dictApi.list('field_category'),
    ])
    typeOptions.value = typeRes.data?.items || []
    categoryOptions.value = catRes.data?.items || []
  } catch {
    // 拦截器已 toast
  }
}

onMounted(() => {
  fetchList()
  loadDictionaries()
})

// ---------- 筛选 ----------

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.label = ''
  query.type = ''
  query.category = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作（占位，T6 实现） ----------

function handleToggle(row, val) {
  // T6 实现
}

function handleEdit(row) {
  if (row.enabled) {
    ElMessageBox.alert('请先禁用该字段，再进行编辑。', '提示', { type: 'warning' })
    return
  }
  router.push(`/fields/${row.id}/edit`)
}

function handleDelete(row) {
  // T6 实现
}

function handleShowRefs(row) {
  // T6 实现
}

// ---------- 辅助 ----------

function rowClassName({ row }) {
  return row.enabled ? '' : 'row-disabled'
}

function typeBadgeType(type) {
  const map = {
    integer: '',
    float: '',
    string: 'success',
    boolean: 'warning',
    select: 'info',
    reference: 'danger',
  }
  return map[type] || 'info'
}

function formatTime(str) {
  if (!str) return ''
  const d = new Date(str)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.field-list {
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

.ref-zero {
  color: #C0C4CC;
}

:deep(.row-disabled) {
  opacity: 0.5;
}
</style>
