<template>
  <div class="constraint-panel">
    <div class="constraint-title">
      <el-tag size="small" type="success">string</el-tag>
      <span class="constraint-label">文本类型 — 约束配置</span>
    </div>
    <div v-if="restricted" class="constraint-warn">
      <el-icon><WarningFilled /></el-icon>
      已被引用，约束只能放宽不能收紧
    </div>
    <el-row :gutter="16" style="margin-bottom: 16px">
      <el-col :span="12">
        <div class="constraint-field">
          <label class="constraint-field-label">最小长度</label>
          <el-input-number
            :model-value="constraints.minLength as number | undefined"
            :controls="false"
            :min="0"
            placeholder="不限"
            style="width: 100%"
            @update:model-value="(v: number | null | undefined) => update('minLength', v)"
          />
        </div>
      </el-col>
      <el-col :span="12">
        <div class="constraint-field">
          <label class="constraint-field-label">最大长度</label>
          <el-input-number
            :model-value="constraints.maxLength as number | undefined"
            :controls="false"
            :min="0"
            placeholder="不限"
            style="width: 100%"
            @update:model-value="(v: number | null | undefined) => update('maxLength', v)"
          />
        </div>
      </el-col>
    </el-row>
    <div class="constraint-field">
      <label class="constraint-field-label">正则校验</label>
      <el-input
        :model-value="constraints.pattern as string | undefined"
        placeholder="选填，如 ^[a-zA-Z\u4e00-\u9fa5]+$"
        @update:model-value="(v: string) => update('pattern', v)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { WarningFilled } from '@element-plus/icons-vue'

const props = defineProps<{
  modelValue?: Record<string, unknown>
  restricted?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, unknown>]
}>()

const constraints = computed(() => (props.modelValue || {}) as Record<string, unknown>)

function update(key: string, val: string | number | null | undefined) {
  const next = { ...constraints.value }
  if (val === null || val === undefined || val === '') {
    delete next[key]
  } else {
    next[key] = val
  }
  emit('update:modelValue', next)
}
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

.constraint-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.constraint-field-label {
  font-size: 13px;
  color: #909399;
}
</style>
