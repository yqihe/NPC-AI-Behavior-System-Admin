<template>
  <!-- Root empty state: depth===0 && no node && not disabled -->
  <div v-if="depth === 0 && !modelValue && !disabled" class="bt-root-empty">
    <el-button type="primary" @click="openSelectorFor('root')">添加根节点</el-button>
  </div>

  <!-- Node card -->
  <div v-else-if="modelValue" class="bt-node-card" :style="{ marginLeft: depth * 24 + 'px' }">
    <!-- Header -->
    <div class="bt-node-header">
      <span class="bt-node-title">
        <el-tag
          :type="categoryTagType(effectiveCategory)"
          size="small"
          class="bt-category-tag"
        >{{ categoryLabel(effectiveCategory) }}</el-tag>
        {{ nodeMeta ? nodeMeta.label : modelValue.type }}
        <span class="bt-type-name">({{ modelValue.type }})</span>
      </span>
      <span v-if="!disabled" class="bt-node-actions">
        <el-button size="small" @click="openSelectorFor('edit')">编辑</el-button>
        <el-button size="small" type="danger" @click="handleDeleteSelf">删除</el-button>
      </span>
    </div>

    <!-- Leaf: inline params form -->
    <template v-if="effectiveCategory === 'leaf'">
      <div class="bt-node-params">
        <div class="bt-params-toggle">
          <el-button size="small" text @click="showParams = !showParams">
            {{ showParams ? '收起参数' : '展开参数' }}
          </el-button>
        </div>
        <!-- Known node type params -->
        <template v-if="nodeMeta">
          <div v-if="showParams" class="bt-params-form">
            <div
              v-for="paramDef in nodeMeta.params"
              :key="paramDef.name"
              class="bt-param-row"
            >
              <label class="bt-param-label">{{ paramDef.label }}</label>
              <div class="bt-param-control">
                <!-- bb_key -->
                <BBKeySelector
                  v-if="paramDef.type === 'bb_key'"
                  :model-value="(modelValue.params[paramDef.name] as string) || ''"
                  :disabled="disabled || !showParams"
                  @update:model-value="(v: string) => updateParam(paramDef.name, v)"
                />
                <!-- select -->
                <el-select
                  v-else-if="paramDef.type === 'select'"
                  :model-value="modelValue.params[paramDef.name]"
                  :disabled="disabled || !showParams"
                  style="width: 100%"
                  @update:model-value="(v: unknown) => updateParam(paramDef.name, v)"
                >
                  <el-option
                    v-for="opt in (paramDef.options || [])"
                    :key="opt"
                    :label="opt"
                    :value="opt"
                  />
                </el-select>
                <!-- float -->
                <el-input-number
                  v-else-if="paramDef.type === 'float'"
                  :model-value="(modelValue.params[paramDef.name] as number) ?? undefined"
                  :disabled="disabled || !showParams"
                  :precision="4"
                  :step="0.1"
                  style="width: 100%"
                  @update:model-value="(v: number | undefined) => updateParam(paramDef.name, v)"
                />
                <!-- integer -->
                <el-input-number
                  v-else-if="paramDef.type === 'integer'"
                  :model-value="(modelValue.params[paramDef.name] as number) ?? undefined"
                  :disabled="disabled || !showParams"
                  :precision="0"
                  :step="1"
                  style="width: 100%"
                  @update:model-value="(v: number | undefined) => updateParam(paramDef.name, v)"
                />
                <!-- bool -->
                <el-select
                  v-else-if="paramDef.type === 'bool'"
                  :model-value="modelValue.params[paramDef.name]"
                  :disabled="disabled || !showParams"
                  style="width: 100%"
                  @update:model-value="(v: unknown) => updateParam(paramDef.name, v)"
                >
                  <el-option :value="true" label="true" />
                  <el-option :value="false" label="false" />
                </el-select>
                <!-- string -->
                <el-input
                  v-else
                  :model-value="(modelValue.params[paramDef.name] as string) || ''"
                  :disabled="disabled || !showParams"
                  @update:model-value="(v: string) => updateParam(paramDef.name, v)"
                />
              </div>
            </div>
            <div v-if="nodeMeta.params.length === 0" class="bt-no-params">
              （无参数）
            </div>
          </div>
        </template>
        <!-- Unknown node type: raw key-value read-only -->
        <template v-else>
          <div v-if="showParams" class="bt-params-form bt-params-unknown">
            <div
              v-for="(val, key) in modelValue.params"
              :key="key"
              class="bt-param-row"
            >
              <label class="bt-param-label">{{ key }}</label>
              <div class="bt-param-control">
                <el-input :model-value="String(val)" disabled />
              </div>
            </div>
            <div v-if="Object.keys(modelValue.params).length === 0" class="bt-no-params">
              （无参数）
            </div>
          </div>
        </template>
      </div>
    </template>

    <!-- Composite: children -->
    <template v-else-if="effectiveCategory === 'composite'">
      <div class="bt-node-children">
        <BtNodeEditor
          v-for="(child, idx) in (modelValue.children || [])"
          :key="idx"
          :model-value="child"
          :node-types="nodeTypes"
          :disabled="disabled"
          :depth="depth + 1"
          @update:model-value="(v) => handleChildUpdate(idx, v)"
        />
        <div v-if="!disabled" class="bt-add-child">
          <el-button size="small" type="primary" plain @click="openSelectorFor('addChild')">
            添加子节点
          </el-button>
        </div>
      </div>
    </template>

    <!-- Decorator: single child slot -->
    <template v-else-if="effectiveCategory === 'decorator'">
      <div class="bt-node-children">
        <template v-if="modelValue.child">
          <BtNodeEditor
            :model-value="modelValue.child"
            :node-types="nodeTypes"
            :disabled="disabled"
            :depth="depth + 1"
            @update:model-value="(v) => handleDecoratorChildUpdate(v)"
          />
        </template>
        <div v-else class="bt-no-child">
          <span class="bt-placeholder">暂无子节点</span>
          <el-button
            v-if="!disabled"
            size="small"
            type="primary"
            plain
            @click="openSelectorFor('setChild')"
          >
            设置子节点
          </el-button>
        </div>
      </div>
    </template>
  </div>

  <!-- BtNodeTypeSelector dialog -->
  <BtNodeTypeSelector
    v-model="selectorVisible"
    :node-types="nodeTypes"
    @select="handleSelectorSelect"
  />
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { BtNodeInternal } from '@/api/btTrees'
import type { BtNodeTypeMeta } from '@/api/btNodeTypes'
import BBKeySelector from '@/components/BBKeySelector.vue'
import BtNodeTypeSelector from '@/components/BtNodeTypeSelector.vue'

defineOptions({ name: 'BtNodeEditor' })

const props = withDefaults(
  defineProps<{
    modelValue: BtNodeInternal | null
    nodeTypes: BtNodeTypeMeta[]
    disabled?: boolean
    depth?: number
  }>(),
  {
    disabled: false,
    depth: 0,
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: BtNodeInternal | null]
}>()

// ─── Selector dialog ───

const selectorVisible = ref(false)
/** What the selector is being opened for */
type SelectorPurpose = 'root' | 'edit' | 'addChild' | 'setChild'
const selectorPurpose = ref<SelectorPurpose>('root')

function openSelectorFor(purpose: SelectorPurpose) {
  selectorPurpose.value = purpose
  selectorVisible.value = true
}

// ─── Computed helpers ───

const typeMap = computed(() => {
  const m = new Map<string, BtNodeTypeMeta>()
  for (const t of props.nodeTypes) {
    m.set(t.type_name, t)
  }
  return m
})

const nodeMeta = computed<BtNodeTypeMeta | undefined>(() => {
  if (!props.modelValue) return undefined
  return typeMap.value.get(props.modelValue.type)
})

/** Effective category: use node's stored category, fall back to 'leaf' for unknown types */
const effectiveCategory = computed<'composite' | 'decorator' | 'leaf'>(() => {
  if (!props.modelValue) return 'leaf'
  return props.modelValue.category ?? 'leaf'
})

const showParams = ref(true)

// ─── Category display helpers ───

function categoryTagType(cat: string): '' | 'warning' | 'success' {
  if (cat === 'decorator') return 'warning'
  if (cat === 'leaf') return 'success'
  return '' // composite → default/blue
}

function categoryLabel(cat: string): string {
  if (cat === 'composite') return '组合'
  if (cat === 'decorator') return '装饰'
  if (cat === 'leaf') return '叶子'
  return cat
}

// ─── Node factory ───

function createNode(meta: BtNodeTypeMeta): BtNodeInternal {
  if (meta.category === 'composite') {
    return { type: meta.type_name, category: 'composite', params: {}, children: [] }
  } else if (meta.category === 'decorator') {
    return { type: meta.type_name, category: 'decorator', params: {}, child: null }
  } else {
    return { type: meta.type_name, category: 'leaf', params: {} }
  }
}

// ─── Selector handler ───

function handleSelectorSelect(meta: BtNodeTypeMeta) {
  const purpose = selectorPurpose.value

  if (purpose === 'root') {
    emit('update:modelValue', createNode(meta))
    return
  }

  if (purpose === 'edit') {
    if (!props.modelValue) return
    emit('update:modelValue', createNode(meta))
    return
  }

  if (purpose === 'addChild') {
    if (!props.modelValue) return
    const cloned = structuredClone(props.modelValue)
    if (!cloned.children) cloned.children = []
    cloned.children.push(createNode(meta))
    emit('update:modelValue', cloned)
    return
  }

  if (purpose === 'setChild') {
    if (!props.modelValue) return
    const cloned = structuredClone(props.modelValue)
    cloned.child = createNode(meta)
    emit('update:modelValue', cloned)
    return
  }
}

// ─── Delete self ───

function handleDeleteSelf() {
  emit('update:modelValue', null)
}

// ─── Composite child update ───

function handleChildUpdate(idx: number, value: BtNodeInternal | null) {
  if (!props.modelValue) return
  const cloned = structuredClone(props.modelValue)
  if (!cloned.children) cloned.children = []
  if (value === null) {
    cloned.children.splice(idx, 1)
  } else {
    cloned.children[idx] = value
  }
  emit('update:modelValue', cloned)
}

// ─── Decorator child update ───

function handleDecoratorChildUpdate(value: BtNodeInternal | null) {
  if (!props.modelValue) return
  const cloned = structuredClone(props.modelValue)
  cloned.child = value
  emit('update:modelValue', cloned)
}

// ─── Leaf param update ───

function updateParam(name: string, value: unknown) {
  if (!props.modelValue) return
  const cloned = structuredClone(props.modelValue)
  cloned.params[name] = value
  emit('update:modelValue', cloned)
}
</script>

<style scoped>
.bt-root-empty {
  display: flex;
  justify-content: center;
  padding: 24px;
  border: 2px dashed #dcdfe6;
  border-radius: 6px;
}

.bt-node-card {
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  margin-bottom: 8px;
  background: #fff;
}

.bt-node-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #f5f7fa;
  border-bottom: 1px solid #ebeef5;
  border-radius: 6px 6px 0 0;
}

.bt-node-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 500;
}

.bt-category-tag {
  flex-shrink: 0;
}

.bt-type-name {
  color: #909399;
  font-size: 12px;
  font-weight: 400;
}

.bt-node-actions {
  display: flex;
  gap: 6px;
}

.bt-node-params {
  padding: 8px 12px;
}

.bt-params-toggle {
  margin-bottom: 6px;
}

.bt-params-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.bt-param-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bt-param-label {
  flex-shrink: 0;
  width: 120px;
  font-size: 13px;
  color: #606266;
  text-align: right;
}

.bt-param-control {
  flex: 1;
}

.bt-no-params {
  font-size: 12px;
  color: #c0c4cc;
  padding: 4px 0;
}

.bt-node-children {
  padding: 8px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.bt-add-child {
  padding-top: 4px;
}

.bt-no-child {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}

.bt-placeholder {
  font-size: 12px;
  color: #c0c4cc;
}
</style>
