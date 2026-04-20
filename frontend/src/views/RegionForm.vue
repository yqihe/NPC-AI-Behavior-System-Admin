<template>
  <div class="region-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="router.push('/regions')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="router.push('/regions')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ isView ? '查看区域' : isCreate ? '新建区域' : '编辑区域' }}
      </span>
    </div>

    <!-- 表单滚动区 -->
    <div class="form-scroll">
      <div class="form-body">

        <!-- Card 1: 基本信息 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-blue"></span>
            <span class="title-text">基本信息</span>
          </div>
          <el-form
            ref="formRef"
            :model="form"
            :rules="rules"
            :disabled="isView"
            label-width="120px"
            label-position="right"
          >
            <!-- 区域标识 -->
            <el-form-item label="区域标识" prop="region_id">
              <template v-if="!isCreate || isView">
                <el-input :model-value="form.region_id" disabled style="width: 100%">
                  <template #prefix>
                    <el-icon><Lock /></el-icon>
                  </template>
                </el-input>
                <div class="field-warn">
                  <el-icon><WarningFilled /></el-icon>
                  创建后不可修改
                </div>
              </template>
              <template v-else>
                <el-input
                  v-model="form.region_id"
                  placeholder="如 village_outskirts（小写字母开头，仅含小写字母、数字、下划线）"
                  style="width: 100%"
                  @blur="checkNameUnique"
                />
                <div v-if="nameStatus === 'checking'" class="field-hint">
                  <el-icon class="is-loading"><Loading /></el-icon>
                  校验中...
                </div>
                <div v-else-if="nameStatus === 'available'" class="field-hint field-hint-success">
                  <el-icon><CircleCheck /></el-icon>
                  标识符可用
                </div>
                <div v-else-if="nameStatus === 'taken'" class="field-hint field-hint-error">
                  <el-icon><CircleClose /></el-icon>
                  {{ nameMessage }}
                </div>
                <div class="field-hint field-gray">
                  格式：小写字母开头，仅含小写字母、数字、下划线
                </div>
              </template>
            </el-form-item>

            <!-- 中文名 -->
            <el-form-item label="中文名" prop="display_name">
              <el-input
                v-model="form.display_name"
                placeholder="如 村庄外围"
                style="width: 100%"
              />
            </el-form-item>

            <!-- 区域类型 -->
            <el-form-item label="区域类型" prop="region_type">
              <el-select
                v-model="form.region_type"
                placeholder="请选择区域类型"
                style="width: 100%"
              >
                <el-option
                  v-for="opt in regionTypeOptions"
                  :key="opt.name"
                  :label="opt.label"
                  :value="opt.name"
                />
              </el-select>
            </el-form-item>
          </el-form>
        </div>

        <!-- Card 2: Spawn 配置 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-green"></span>
            <span class="title-text">Spawn 配置</span>
            <span class="title-hint">控制该区域初始化时批量生成哪些 NPC</span>
          </div>

          <div v-if="form.spawn_table.length === 0" class="spawn-empty">
            <el-empty description="暂无 Spawn 配置">
              <el-button v-if="!isView" type="primary" @click="handleAddEntry">
                <el-icon><Plus /></el-icon>
                添加 Spawn Entry
              </el-button>
            </el-empty>
          </div>

          <div v-else class="spawn-list">
            <el-card
              v-for="(entry, idx) in form.spawn_table"
              :key="idx"
              class="spawn-entry-card"
              shadow="never"
            >
              <template #header>
                <div class="entry-header">
                  <span class="entry-header-title">Entry {{ idx + 1 }}</span>
                  <el-button
                    v-if="!isView"
                    type="danger"
                    link
                    size="small"
                    @click="handleRemoveEntry(idx)"
                  >
                    <el-icon><Delete /></el-icon>
                    删除此 entry
                  </el-button>
                </div>
              </template>

              <el-form
                :model="entry"
                :disabled="isView"
                label-width="120px"
                label-position="right"
              >
                <!-- NPC 引用 -->
                <el-form-item label="NPC 引用" required>
                  <el-select
                    v-model="entry.template_ref"
                    placeholder="请选择 NPC"
                    filterable
                    style="width: 100%"
                    :class="{ 'entry-error': entryErrors[idx]?.template_ref }"
                  >
                    <el-option
                      v-for="n in npcOptions"
                      :key="n.name"
                      :label="`${n.label}（${n.name}）`"
                      :value="n.name"
                    />
                  </el-select>
                  <div v-if="entryErrors[idx]?.template_ref" class="field-hint field-hint-error">
                    <el-icon><CircleClose /></el-icon>
                    {{ entryErrors[idx].template_ref }}
                  </div>
                </el-form-item>

                <!-- 数量 -->
                <el-form-item label="数量" required>
                  <el-input-number
                    v-model="entry.count"
                    :min="1"
                    :step="1"
                    style="width: 160px"
                  />
                </el-form-item>

                <!-- 游荡半径 -->
                <el-form-item label="游荡半径">
                  <el-input-number
                    v-model="entry.wander_radius"
                    :min="0"
                    :step="0.1"
                    :precision="1"
                    style="width: 160px"
                  />
                  <span class="unit-suffix">米</span>
                </el-form-item>

                <!-- 重生间隔（v3 占位） -->
                <el-form-item label="重生间隔">
                  <el-input-number
                    v-model="entry.respawn_seconds"
                    :min="0"
                    :step="1"
                    :precision="0"
                    style="width: 160px"
                  />
                  <span class="unit-suffix">秒</span>
                  <div class="field-hint field-gray">
                    <el-icon><InfoFilled /></el-icon>
                    Server v3+ 生效，当前仅保存不调度
                  </div>
                </el-form-item>

                <!-- Spawn 坐标点 -->
                <el-form-item label="Spawn 坐标">
                  <div class="points-wrap">
                    <el-table
                      :data="entry.spawn_points"
                      size="small"
                      border
                      style="width: 100%"
                    >
                      <el-table-column label="#" width="60" align="center">
                        <template #default="{ $index }">{{ $index + 1 }}</template>
                      </el-table-column>
                      <el-table-column label="X" min-width="140">
                        <template #default="{ row }">
                          <el-input-number
                            v-model="row.x"
                            :step="0.1"
                            :precision="1"
                            :controls="false"
                            style="width: 100%"
                          />
                        </template>
                      </el-table-column>
                      <el-table-column label="Z" min-width="140">
                        <template #default="{ row }">
                          <el-input-number
                            v-model="row.z"
                            :step="0.1"
                            :precision="1"
                            :controls="false"
                            style="width: 100%"
                          />
                        </template>
                      </el-table-column>
                      <el-table-column label="操作" width="80" align="center" v-if="!isView">
                        <template #default="{ $index }">
                          <el-button
                            type="danger"
                            link
                            size="small"
                            @click="handleRemovePoint(idx, $index)"
                          >
                            删除
                          </el-button>
                        </template>
                      </el-table-column>
                    </el-table>

                    <div class="points-actions">
                      <el-button v-if="!isView" size="small" @click="handleAddPoint(idx)">
                        <el-icon><Plus /></el-icon>
                        添加坐标
                      </el-button>
                      <span
                        v-if="entryErrors[idx]?.spawn_points"
                        class="field-hint field-hint-error points-error"
                      >
                        <el-icon><CircleClose /></el-icon>
                        {{ entryErrors[idx].spawn_points }}
                      </span>
                      <span v-else class="field-hint field-gray points-count-hint">
                        共 {{ entry.spawn_points.length }} 个坐标
                        <span v-if="entry.spawn_points.length < entry.count" class="points-count-warn">
                          （需至少 {{ entry.count }} 个）
                        </span>
                      </span>
                    </div>
                  </div>
                </el-form-item>
              </el-form>
            </el-card>

            <div v-if="!isView" class="add-entry-row">
              <el-button type="primary" plain @click="handleAddEntry">
                <el-icon><Plus /></el-icon>
                添加 Spawn Entry
              </el-button>
            </div>
          </div>
        </div>

      </div>
    </div>

    <!-- 底部操作栏（查看模式隐藏） -->
    <div v-if="!isView" class="form-footer">
      <el-button @click="router.push('/regions')">取消</el-button>
      <el-button
        type="primary"
        :loading="submitting"
        :disabled="!canSubmit"
        @click="handleSubmit"
      >
        保存
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import {
  ArrowLeft, Lock, WarningFilled, Loading,
  CircleCheck, CircleClose, Plus, Delete, InfoFilled,
} from '@element-plus/icons-vue'
import { regionApi, REGION_ERR } from '@/api/regions'
import type { SpawnEntry } from '@/api/regions'
import { npcApi } from '@/api/npc'
import type { NPCListItem } from '@/api/npc'
import type { DictionaryItem } from '@/api/dictionaries'
import type { BizError } from '@/api/request'

const route = useRoute()
const router = useRouter()
const isCreate = route.meta.isCreate as boolean
const isView = (route.meta.isView as boolean) || false

const formRef = ref<FormInstance>()
const submitting = ref(false)
const nameStatus = ref<'' | 'checking' | 'available' | 'taken'>('')
const nameMessage = ref('')
const version = ref(0)

interface FormState {
  region_id: string
  display_name: string
  region_type: string
  spawn_table: SpawnEntry[]
}

const form = reactive<FormState>({
  region_id: '',
  display_name: '',
  region_type: '',
  spawn_table: [],
})

interface EntryError {
  template_ref?: string
  spawn_points?: string
}
const entryErrors = ref<Record<number, EntryError>>({})

const regionTypeOptions = ref<DictionaryItem[]>([])
const npcOptions = ref<NPCListItem[]>([])

const namePattern = /^[a-z][a-z0-9_]*$/

const rules = {
  region_id: [
    { required: true, message: '请输入区域标识', trigger: 'blur' },
    {
      pattern: namePattern,
      message: '小写字母开头，仅含小写字母、数字、下划线',
      trigger: 'blur',
    },
  ],
  display_name: [
    { required: true, message: '请输入中文名', trigger: 'blur' },
  ],
  region_type: [
    { required: true, message: '请选择区域类型', trigger: 'change' },
  ],
}

// ─── 初始化 ───

onMounted(async () => {
  await Promise.all([loadRegionTypes(), loadNPCs()])
  if (!isCreate) await loadDetail()
})

async function loadRegionTypes() {
  try {
    const res = await regionApi.getRegionTypeOptions()
    regionTypeOptions.value = res.data?.items || []
  } catch {
    // 拦截器已 toast
  }
}

async function loadNPCs() {
  try {
    const res = await npcApi.list({ enabled: true, page: 1, page_size: 1000 })
    npcOptions.value = res.data?.items || []
  } catch {
    // 拦截器已 toast
  }
}

async function loadDetail() {
  const id = Number(route.params.id)
  try {
    const res = await regionApi.detail(id)
    const data = res.data
    form.region_id = data.region_id
    form.display_name = data.display_name
    form.region_type = data.region_type
    form.spawn_table = Array.isArray(data.spawn_table) ? data.spawn_table : []
    version.value = data.version
  } catch (err: unknown) {
    if ((err as BizError).code === REGION_ERR.NOT_FOUND) {
      ElMessage.error('区域不存在')
      router.push('/regions')
    }
  }
}

// ─── region_id 唯一性校验 ───

async function checkNameUnique() {
  if (!form.region_id || !namePattern.test(form.region_id)) {
    nameStatus.value = ''
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await regionApi.checkName(form.region_id)
    if (res.data?.available) {
      nameStatus.value = 'available'
      nameMessage.value = ''
    } else {
      nameStatus.value = 'taken'
      nameMessage.value = res.data?.message || '标识符已被使用'
    }
  } catch {
    nameStatus.value = ''
  }
}

// ─── Spawn Entry 操作 ───

function createEmptyEntry(): SpawnEntry {
  return {
    template_ref: '',
    count: 1,
    spawn_points: [{ x: 0, z: 0 }],
    wander_radius: 0,
    respawn_seconds: 0,
  }
}

function handleAddEntry() {
  form.spawn_table.push(createEmptyEntry())
}

function handleRemoveEntry(idx: number) {
  form.spawn_table.splice(idx, 1)
  delete entryErrors.value[idx]
  // 重排 entryErrors key 避免错位
  const next: Record<number, EntryError> = {}
  Object.entries(entryErrors.value).forEach(([k, v]) => {
    const ki = Number(k)
    next[ki > idx ? ki - 1 : ki] = v
  })
  entryErrors.value = next
}

function handleAddPoint(entryIdx: number) {
  form.spawn_table[entryIdx].spawn_points.push({ x: 0, z: 0 })
}

function handleRemovePoint(entryIdx: number, pointIdx: number) {
  form.spawn_table[entryIdx].spawn_points.splice(pointIdx, 1)
}

// ─── 提交前校验 ───

const canSubmit = computed(() => {
  // 所有 entry spawn_points.length >= count
  return form.spawn_table.every((e) => e.spawn_points.length >= e.count)
})

function validateSpawnTable(): boolean {
  const errors: Record<number, EntryError> = {}
  let hasError = false
  form.spawn_table.forEach((e, idx) => {
    const err: EntryError = {}
    if (!e.template_ref) {
      err.template_ref = '请选择 NPC 引用'
      hasError = true
    }
    if (e.spawn_points.length < e.count) {
      err.spawn_points = `Spawn 坐标数 (${e.spawn_points.length}) 少于数量 (${e.count})`
      hasError = true
    }
    if (Object.keys(err).length > 0) errors[idx] = err
  })
  entryErrors.value = errors
  return !hasError
}

// ─── 提交 ───

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.warning('区域标识已被使用，请更换')
    return
  }

  if (!validateSpawnTable()) {
    ElMessage.warning('Spawn 配置存在错误，请检查后重试')
    return
  }

  submitting.value = true
  try {
    if (isCreate) {
      await regionApi.create({
        region_id: form.region_id,
        display_name: form.display_name,
        region_type: form.region_type,
        spawn_table: form.spawn_table,
      })
      ElMessage.success('创建成功，区域默认为禁用状态，确认无误后请手动启用')
    } else {
      await regionApi.update({
        id: Number(route.params.id),
        version: version.value,
        display_name: form.display_name,
        region_type: form.region_type,
        spawn_table: form.spawn_table,
      })
      ElMessage.success('保存成功')
    }
    router.push('/regions')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === REGION_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他人修改，请刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (bizErr.code === REGION_ERR.ID_EXISTS || bizErr.code === REGION_ERR.ID_INVALID) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === REGION_ERR.NOT_FOUND) {
      ElMessage.error('区域不存在')
      router.push('/regions')
      return
    }
    if (bizErr.code === REGION_ERR.EDIT_NOT_DISABLED) {
      ElMessage.warning('请先禁用该区域后再编辑')
      return
    }
    if (
      bizErr.code === REGION_ERR.TEMPLATE_REF_NOT_FOUND ||
      bizErr.code === REGION_ERR.TEMPLATE_REF_DISABLED ||
      bizErr.code === REGION_ERR.SPAWN_ENTRY_INVALID ||
      bizErr.code === REGION_ERR.TYPE_INVALID
    ) {
      // 后端返的 spawn_table 相关错，高亮到 entry（无法精确定位到具体 entry 时直接 toast）
      ElMessage.error(bizErr.message)
      return
    }
    // 其他错误拦截器已 toast
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.field-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}
.field-hint-success { color: #67C23A; }
.field-hint-error   { color: #F56C6C; }
.field-gray         { color: #909399; }

.field-warn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #E6A23C;
}

.title-hint {
  margin-left: 12px;
  font-size: 12px;
  color: #909399;
  font-weight: normal;
}

.spawn-empty {
  padding: 24px 0;
}

.spawn-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.spawn-entry-card {
  border: 1px solid #DCDFE6;
}

.spawn-entry-card :deep(.el-card__header) {
  padding: 12px 20px;
  background: #F5F7FA;
}

.spawn-entry-card :deep(.el-card__body) {
  padding: 20px 20px 0;
}

.entry-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.entry-header-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.unit-suffix {
  margin-left: 8px;
  font-size: 13px;
  color: #909399;
}

.points-wrap {
  width: 100%;
}

.points-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
}

.points-count-hint {
  margin-top: 0;
}

.points-count-warn {
  color: #F56C6C;
  margin-left: 4px;
}

.points-error {
  margin-top: 0;
}

.add-entry-row {
  display: flex;
  justify-content: center;
  padding: 8px 0;
}

.entry-error :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px #F56C6C inset;
}
</style>
