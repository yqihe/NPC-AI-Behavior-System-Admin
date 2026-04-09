<template>
  <div class="constraint-panel">
    <div class="constraint-title">
      <el-tag size="small">{{ typeName }}</el-tag>
      <span class="constraint-label">{{ typeName === 'integer' ? '整数' : '浮点数' }}类型 — 约束配置</span>
    </div>
    <div v-if="restricted" class="constraint-warn">
      <el-icon><WarningFilled /></el-icon>
      已被引用，约束只能放宽不能收紧
    </div>
    <el-row :gutter="16">
      <el-col :span="8">
        <div class="constraint-field">
          <label class="constraint-field-label">最小值</label>
          <el-input-number
            :model-value="constraints.minimum"
            :controls="false"
            placeholder="不限"
            style="width: 100%"
            @update:model-value="(v) => update('minimum', v)"
          />
        </div>
      </el-col>
      <el-col :span="8">
        <div class="constraint-field">
          <label class="constraint-field-label">最大值</label>
          <el-input-number
            :model-value="constraints.maximum"
            :controls="false"
            placeholder="不限"
            style="width: 100%"
            @update:model-value="(v) => update('maximum', v)"
          />
        </div>
      </el-col>
      <el-col :span="8">
        <div class="constraint-field">
          <label class="constraint-field-label">步长</label>
          <el-input-number
            :model-value="constraints.step"
            :controls="false"
            :min="0"
            placeholder="默认 1"
            style="width: 100%"
            @update:model-value="(v) => update('step', v)"
          />
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { WarningFilled } from '@element-plus/icons-vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  restricted: { type: Boolean, default: false },
  typeName: { type: String, default: 'integer' },
})

const emit = defineEmits(['update:modelValue'])

const constraints = computed(() => props.modelValue || {})

function update(key, val) {
  const next = { ...constraints.value }
  if (val === null || val === undefined) {
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
