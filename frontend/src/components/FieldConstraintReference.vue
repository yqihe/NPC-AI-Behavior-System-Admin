<template>
  <div class="constraint-panel">
    <div class="constraint-title">
      <el-tag size="small" type="danger">reference</el-tag>
      <span class="constraint-label">引用类型 — 约束配置</span>
    </div>
    <div v-if="restricted" class="constraint-warn">
      <el-icon><WarningFilled /></el-icon>
      已被引用，约束只能放宽不能收紧
    </div>

    <!-- 引用列表头 -->
    <div class="ref-header">
      <span class="ref-label">引用字段列表</span>
      <el-link type="primary" :underline="false" @click="showAddDropdown = true">
        <el-icon><Plus /></el-icon>
        添加引用
      </el-link>
    </div>

    <!-- 添加引用下拉 -->
    <div v-if="showAddDropdown" class="add-ref-row">
      <el-select
        v-model="addRefId"
        placeholder="选择要引用的字段（仅启用字段）"
        filterable
        style="flex: 1"
      >
        <el-option
          v-for="f in availableFields"
          :key="f.id"
          :label="`${f.label} (${f.name})`"
          :value="f.id"
        />
      </el-select>
      <el-button type="primary" size="small" :disabled="!addRefId" @click="addRef">确定</el-button>
      <el-button size="small" @click="showAddDropdown = false; addRefId = null">取消</el-button>
    </div>

    <!-- 引用列表 -->
    <div class="ref-list">
      <div
        v-for="(item, idx) in refFields"
        :key="item.id"
        class="ref-item"
      >
        <el-icon class="grip-icon"><Rank /></el-icon>
        <el-tag size="small" :type="item.type === 'reference' ? 'danger' : ''">
          {{ item.type_label || item.type }}
        </el-tag>
        <span class="ref-name">{{ item.name }}</span>
        <span class="ref-label-text">{{ item.label }}</span>
        <span class="ref-spacer"></span>
        <el-icon class="del-icon" @click="removeRef(idx)"><Close /></el-icon>
      </div>
      <div v-if="refFields.length === 0" class="ref-empty">
        暂无引用字段，请点击「添加引用」
      </div>
    </div>

    <!-- 展开预览 -->
    <div v-if="refFields.length > 0" class="expand-box">
      <div class="expand-title">
        <el-icon><View /></el-icon>
        <span>展开预览（模板勾选时实际包含的字段）</span>
      </div>
      <p class="expand-list">
        {{ previewText }}
      </p>
    </div>

    <!-- 循环引用警告 -->
    <div class="cycle-warn">
      <el-icon><WarningFilled /></el-icon>
      引用其他 reference 字段时，系统自动检测循环引用
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { WarningFilled, Plus, Close, Rank, View } from '@element-plus/icons-vue'
import { fieldApi } from '@/api/fields'
import type { FieldListItem } from '@/api/fields'

interface RefFieldItem {
  id: number
  name: string
  label: string
  type: string
  type_label?: string
}

interface RefConstraints {
  ref_fields?: RefFieldItem[]
  [key: string]: unknown
}

const props = defineProps<{
  modelValue?: RefConstraints
  restricted?: boolean
  currentFieldId?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: RefConstraints]
}>()

const constraints = computed((): RefConstraints => props.modelValue || {})
const refFields = computed((): RefFieldItem[] => constraints.value.ref_fields || [])

const showAddDropdown = ref(false)
const addRefId = ref<number | null>(null)
const enabledFields = ref<FieldListItem[]>([])

// 加载可选字段（仅启用状态）
async function loadEnabledFields() {
  try {
    const res = await fieldApi.list({ enabled: true, page: 1, page_size: 1000 })
    enabledFields.value = res.data?.items || []
  } catch {
    // 拦截器已 toast
  }
}

// 排除自身和已选字段
const availableFields = computed(() => {
  const selectedIds = new Set(refFields.value.map((f) => f.id))
  return enabledFields.value.filter(
    (f) => f.id !== props.currentFieldId && !selectedIds.has(f.id),
  )
})

const previewText = computed(() => {
  return refFields.value.map((f) => `${f.name} (${f.label})`).join('、') || ''
})

function addRef() {
  if (!addRefId.value) return
  const field = enabledFields.value.find((f) => f.id === addRefId.value)
  if (!field) return
  const newRefFields = [
    ...refFields.value,
    {
      id: field.id,
      name: field.name,
      label: field.label,
      type: field.type,
      type_label: field.type_label,
    },
  ]
  emit('update:modelValue', { ...constraints.value, ref_fields: newRefFields })
  addRefId.value = null
  showAddDropdown.value = false
}

function removeRef(idx: number) {
  const newRefFields = refFields.value.filter((_, i) => i !== idx)
  emit('update:modelValue', { ...constraints.value, ref_fields: newRefFields })
}

// 首次显示添加下拉时加载字段列表
watch(showAddDropdown, (val) => {
  if (val && enabledFields.value.length === 0) {
    loadEnabledFields()
  }
})
</script>

<style scoped>
.constraint-panel {
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 24px;
  background: #fff;
}

.constraint-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.constraint-label {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.constraint-warn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 12px;
  font-size: 12px;
  color: #E6A23C;
}

.ref-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.ref-label {
  font-size: 13px;
  color: #909399;
}

.add-ref-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.ref-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.ref-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: #F5F7FA;
  border-radius: 4px;
}

.grip-icon {
  color: #C0C4CC;
  font-size: 14px;
  cursor: grab;
}

.ref-name {
  font-size: 13px;
  font-weight: 500;
  color: #303133;
}

.ref-label-text {
  font-size: 13px;
  color: #909399;
}

.ref-spacer {
  flex: 1;
}

.del-icon {
  color: #F56C6C;
  cursor: pointer;
  font-size: 14px;
}

.ref-empty {
  text-align: center;
  color: #C0C4CC;
  font-size: 13px;
  padding: 12px 0;
}

.expand-box {
  background: #F5F7FA;
  border: 1px solid #E4E7ED;
  border-radius: 4px;
  padding: 12px 14px;
  margin-bottom: 16px;
}

.expand-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
}

.expand-list {
  font-size: 12px;
  color: #606266;
  line-height: 1.6;
  margin: 0;
}

.cycle-warn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: #FDF6EC;
  border-radius: 4px;
  font-size: 12px;
  color: #E6A23C;
}
</style>
