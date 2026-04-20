<template>
  <el-select
    :model-value="modelValue"
    filterable
    clearable
    :disabled="disabled"
    placeholder="选择 BB Key"
    style="width: 100%"
    @update:model-value="handleChange"
  >
    <el-option-group v-if="npcOptions.length > 0" label="NPC 字段">
      <el-option
        v-for="f in npcOptions"
        :key="f.name"
        :label="`${f.name} (${f.label})`"
        :value="f.name"
      />
    </el-option-group>
    <el-option-group v-if="schemaOptions.length > 0" label="事件扩展字段">
      <el-option
        v-for="f in schemaOptions"
        :key="f.name"
        :label="`${f.name} (${f.label})`"
        :value="f.name"
      />
    </el-option-group>
    <!-- 第三路：运行时 Key（按 group_name 次级分组，对齐 Server keys.go 11 组） -->
    <el-option-group
      v-for="grp in runtimeGroupedOptions"
      :key="grp.groupName"
      :label="`运行时 Key — ${grp.groupLabel}`"
    >
      <el-option
        v-for="f in grp.keys"
        :key="f.name"
        :label="`${f.name} (${f.label})`"
        :value="f.name"
      />
    </el-option-group>
    <el-option
      v-if="npcOptions.length === 0 && schemaOptions.length === 0 && runtimeOptions.length === 0"
      label="暂无可用 BB Key（请先配置字段、事件类型或运行时 Key）"
      value=""
      disabled
    />
  </el-select>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { fieldApi } from '@/api/fields'
import { eventTypeApi } from '@/api/eventTypes'
import { runtimeBbKeyApi, RUNTIME_BB_KEY_GROUPS } from '@/api/runtimeBbKeys'

/** 规范化后的 BB Key 条目，供 FsmConditionEditor 使用 */
export interface BBKeyField {
  name: string
  label: string
  /** 规范化类型：integer / float / string / bool / select / reference */
  type: string
}

/** 运行时 Key 扩展条目（带 group_name，供下拉次级分组） */
interface RuntimeBBKeyField extends BBKeyField {
  group_name: string
}

const props = defineProps<{
  modelValue: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** 选中已知字段时携带规范化 BBKeyField；清空时为 null */
  'field-selected': [field: BBKeyField | null]
}>()

const npcOptions = ref<BBKeyField[]>([])
const schemaOptions = ref<BBKeyField[]>([])
const runtimeOptions = ref<RuntimeBBKeyField[]>([])

/**
 * 规范化字段类型名：
 *  - NPC 字段用 'boolean'，条件编辑器期望 'bool'  → 统一为 'bool'
 *  - 事件扩展字段用 'int'，条件编辑器期望 'integer' → 统一为 'integer'
 *  - 运行时 Key 已是 integer/float/string/bool，无需转换
 */
function normalizeType(raw: string): string {
  if (raw === 'boolean') return 'bool'
  if (raw === 'int') return 'integer'
  return raw
}

/** 按 RUNTIME_BB_KEY_GROUPS 声明顺序对 runtime 分组，空组自动跳过 */
const runtimeGroupedOptions = computed(() => {
  const byGroup = new Map<string, RuntimeBBKeyField[]>()
  for (const f of runtimeOptions.value) {
    const list = byGroup.get(f.group_name) || []
    list.push(f)
    byGroup.set(f.group_name, list)
  }
  return RUNTIME_BB_KEY_GROUPS
    .map((g) => ({
      groupName: g.value,
      groupLabel: g.label,
      keys: byGroup.get(g.value) || [],
    }))
    .filter((g) => g.keys.length > 0)
})

onMounted(async () => {
  try {
    const [fieldRes, schemaRes, runtimeRes] = await Promise.all([
      fieldApi.list({ bb_exposed: true, enabled: true, page: 1, page_size: 200 }),
      eventTypeApi.schemaList({ enabled: true }),
      runtimeBbKeyApi.list({ enabled: true, page: 1, page_size: 200 }),
    ])
    npcOptions.value = (fieldRes.data?.items || []).map((f) => ({
      name: f.name,
      label: f.label,
      type: normalizeType(f.type),
    }))
    schemaOptions.value = (schemaRes.data?.items || []).map((s) => ({
      name: s.field_name,
      label: s.field_label,
      type: normalizeType(s.field_type),
    }))
    runtimeOptions.value = (runtimeRes.data?.items || []).map((r) => ({
      name: r.name,
      label: r.label,
      type: normalizeType(r.type),
      group_name: r.group_name,
    }))
    // 初始值已有 key 时，补发 field-selected 让父组件获知字段类型
    if (props.modelValue) {
      const all: BBKeyField[] = [
        ...npcOptions.value,
        ...schemaOptions.value,
        ...runtimeOptions.value,
      ]
      const matched = all.find((f) => f.name === props.modelValue)
      emit('field-selected', matched || null)
    }
  } catch {
    // 拦截器已 toast
  }
})

function handleChange(val: string) {
  emit('update:modelValue', val || '')
  const all: BBKeyField[] = [
    ...npcOptions.value,
    ...schemaOptions.value,
    ...runtimeOptions.value,
  ]
  const matched = all.find((f) => f.name === val)
  emit('field-selected', matched || null)
}
</script>
