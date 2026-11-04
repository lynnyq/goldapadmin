<template>
  <el-dialog
    :model-value="modelValue"
    title="新建条目"
    width="860px"
    top="6vh"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <!-- 信息区：父 DN + 实时 DN 预览 — 对标 phpLDAPadmin 创建页的 Container/DN 显示 -->
    <div class="info-panel">
      <div class="info-row">
        <span class="info-label">父 DN</span>
        <span class="info-value mono">{{ props.parentDn || '（未选择父条目，请先在目录树中选择）' }}</span>
      </div>
      <div class="info-row">
        <span class="info-label">完整 DN</span>
        <span class="info-value mono" :class="{ 'is-preview': !dnPreview }">{{ dnPreview || dnPreviewPlaceholder }}</span>
      </div>
    </div>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="150px" label-position="right">
      <el-form-item label="创建方式">
        <el-radio-group v-model="form.mode" @change="onModeChange">
          <el-radio value="template">使用模板</el-radio>
          <el-radio value="custom">自定义</el-radio>
        </el-radio-group>
      </el-form-item>

      <template v-if="form.mode === 'template'">
        <el-form-item label="选择模板">
          <el-select v-model="form.template" style="width: 100%" @change="onTemplateChange">
            <el-option
              v-for="t in templates"
              :key="t.name"
              :label="t.name"
              :value="t.name"
              :disabled="!isTemplateAvailable(t)"
            />
          </el-select>
          <div v-if="currentTemplate?.hint" class="template-hint">{{ currentTemplate.hint }}</div>
          <!-- 对标 phpLDAPadmin：模板创建时 objectClass 由模板自动设定，完全隐藏不出现在表单中，
               仅以说明文字告知；不作为表单项、不参与校验 -->
          <div class="template-hint">
            objectClass：<span class="mono">{{ form.objectClasses.join(', ') }}</span>（由模板自动设定，创建时写入条目）
          </div>
          <div v-if="unavailableHint" class="schema-warn">
            <el-icon><WarningFilled /></el-icon>
            {{ unavailableHint }}
          </div>
        </el-form-item>

        <!-- 对标 phpLDAPadmin drawRDNChooser：模板定义 RDN 时属性固定，不提供下拉 -->
        <el-form-item label="RDN 属性">
          <div class="rdn-fixed">
            <span class="rdn-fixed-name mono">{{ form.rdnAttr }}</span>
            <el-tag size="small" class="rdn-tag">RDN</el-tag>
            <span class="rdn-fixed-hint">由模板固定</span>
          </div>
        </el-form-item>
      </template>

      <template v-else>
        <el-form-item label="objectClass">
          <el-select
            v-model="form.objectClasses"
            multiple
            filterable
            allow-create
            default-first-option
            collapse-tags
            collapse-tags-tooltip
            style="width: 100%"
            placeholder="选择或输入 objectClass（选填，选中后自动加载属性）"
            @change="onObjectClassChange"
          >
            <el-option v-for="oc in objectClassList" :key="oc.name" :label="oc.name" :value="oc.name" />
          </el-select>
        </el-form-item>

        <!-- 自定义模式：RDN 候选 = 条目属性（排除 objectClass）— 对标 phpLDAPadmin drawRDNChooser -->
        <el-form-item label="RDN 属性" prop="rdnAttr">
          <el-select v-model="form.rdnAttr" filterable style="width: 100%">
            <el-option
              v-for="a in rdnCandidates"
              :key="a"
              :label="`${shortLabel(a)} (${a})`"
              :value="a"
            />
          </el-select>
        </el-form-item>
      </template>

      <el-form-item label="RDN 值" prop="rdnValue">
        <el-input v-model="form.rdnValue" :placeholder="`条目 RDN，如 ${form.rdnAttr}=zhangsan 的值`" />
      </el-form-item>

      <el-divider content-position="left">属性</el-divider>

      <div v-for="(field, idx) in form.fields" :key="idx" class="field-row">
        <el-form-item
          :prop="`fields.${idx}.value`"
          :rules="isNumberField(field.name) ? numberRules(field) : field.must ? [{ required: true, message: `${shortLabel(field.name)} 为必填属性`, trigger: 'blur' }] : []"
        >
          <template #label>
            <span :title="field.name" class="field-label-text">{{ shortLabel(field.name) }}</span>
            <el-tag v-if="field.name.toLowerCase() === form.rdnAttr.toLowerCase()" size="small" class="rdn-tag">RDN</el-tag>
          </template>
          <div class="field-input-row">
            <!-- loginShell 下拉选择常用 shell — 对标 phpLDAPadmin -->
            <el-select
              v-if="isShellField(field.name)"
              v-model="field.value"
              filterable
              allow-create
              class="shell-select"
              placeholder="选择登录 Shell"
            >
              <el-option v-for="s in commonShells" :key="s" :label="s" :value="s" />
            </el-select>
            <el-input
              v-else
              v-model="field.value"
              :placeholder="isNumberField(field.name) ? '非负整数' : field.must ? '必填' : '可选'"
              :type="isPasswordField(field.name) ? 'password' : 'text'"
              :show-password="isPasswordField(field.name)"
              :inputmode="isNumberField(field.name) ? 'numeric' : undefined"
            />
            <!-- 自增 ID 重新分配 — 对标 phpLDAPadmin GetNextNumber -->
            <el-tooltip v-if="isAutoIDField(field.name)" content="重新分配下一个可用 ID" placement="top">
              <el-button :icon="Refresh" @click="fillAutoID(field)" />
            </el-tooltip>
            <!-- gidNumber 从已有组选择 — 对标 phpLDAPadmin PickList；仅在创建含 posixAccount 的条目时显示，
                 POSIX 组模板的 gidNumber 是自增编号，不需要从已有组选择 -->
            <el-tooltip v-if="isGidFieldForAccount(field.name)" content="从已有 POSIX 组选择 gidNumber" placement="top">
              <el-button :icon="Folder" @click="openGidGroupPick(field)" />
            </el-tooltip>
            <!-- 密码 helper — 对标 phpLDAPadmin 的哈希算法选择 -->
            <el-select
              v-if="isPasswordField(field.name)"
              v-model="field.algo"
              class="pwd-algo"
              size="default"
            >
              <el-option label="SSHA" value="SSHA" />
              <el-option label="SHA" value="SHA" />
              <el-option label="MD5" value="MD5" />
              <el-option label="明文" value="PLAIN" />
            </el-select>
          </div>
        </el-form-item>
        <el-button
          v-if="!field.must"
          link
          type="danger"
          class="field-remove"
          @click="removeField(idx)"
        >
          移除
        </el-button>
      </div>

      <div class="form-actions">
        <el-button @click="showAddField = true">
          <el-icon><Plus /></el-icon>&nbsp;添加属性
        </el-button>
        <el-button @click="addValue">
          <el-icon><Plus /></el-icon>&nbsp;为 {{ form.rdnAttr }} 添加值
        </el-button>
      </div>
    </el-form>

    <!-- 组初始成员选择 — 对标 phpLDAPadmin posixGroup 模板的 MultiList -->
    <div v-if="hasMemberUidField" class="member-section">
      <div class="ms-head">
        <span class="ms-title">组成员 (memberUid)</span>
        <el-button type="primary" link @click="openUserPick">
          <el-icon><User /></el-icon>&nbsp;从用户选择
        </el-button>
      </div>
      <div v-if="pickedMemberUids.length" class="ms-chips">
        <el-tag v-for="u in pickedMemberUids" :key="u" closable @close="removePickedMember(u)">
          {{ u }}
        </el-tag>
      </div>
      <div v-else class="ms-empty">尚未选择成员，组创建后也可在成员属性中添加</div>
    </div>

    <!-- gidNumber 从已有 POSIX 组选择 — 对标 phpLDAPadmin PickList -->
    <el-dialog v-model="showGidPick" title="选择组（gidNumber）" width="600px" append-to-body>
      <div class="dlg-body">
        <el-input v-model="gidPickFilter" placeholder="输入组名过滤" clearable />
        <el-table
          :data="filteredGidGroups"
          max-height="320"
          highlight-current-row
          @row-click="(row: GidGroup) => confirmGidPick(row)"
        >
          <el-table-column prop="cn" label="组名" width="180" />
          <el-table-column prop="gidNumber" label="gidNumber" width="120" />
          <el-table-column prop="dn" label="DN" show-overflow-tooltip />
        </el-table>
      </div>
      <template #footer>
        <el-button @click="showGidPick = false">取消</el-button>
        <div class="picker-hint">点击行即选中并填入 gidNumber</div>
      </template>
    </el-dialog>

    <!-- 用户选择器：为 memberUid 挑选组成员 -->
    <el-dialog v-model="showUserPick" title="选择组成员" width="640px" append-to-body>
      <div class="dlg-body">
        <el-input v-model="userPickFilter" placeholder="输入 uid 或姓名过滤" clearable />
        <el-table
          :data="filteredPickUsers"
          max-height="320"
          @selection-change="(rows: PickUser[]) => userPickSelection = rows"
        >
          <el-table-column type="selection" width="45" />
          <el-table-column prop="uid" label="UID" width="140" />
          <el-table-column prop="cn" label="姓名" width="160" />
          <el-table-column prop="dn" label="DN" show-overflow-tooltip />
        </el-table>
      </div>
      <template #footer>
        <el-button @click="showUserPick = false">取消</el-button>
        <el-button type="primary" :disabled="!userPickSelection.length" @click="confirmUserPick">
          添加所选（{{ userPickSelection.length }}）
        </el-button>
      </template>
    </el-dialog>

    <!-- 添加属性 -->
    <el-dialog v-model="showAddField" title="添加属性" width="400px" append-to-body>
      <el-form label-width="80px">
        <el-form-item label="属性名">
          <el-select v-model="newField.name" filterable allow-create style="width: 100%">
            <el-option v-for="a in availableAttrs" :key="a" :label="`${shortLabel(a)} (${a})`" :value="a" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddField = false">取消</el-button>
        <el-button type="primary" @click="addField">添加</el-button>
      </template>
    </el-dialog>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="creating" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
/**
 * 新建条目对话框组件 — 在指定父 DN 下创建新的 LDAP 条目
 * 提供模板模式（预设 objectClass + 必填属性）和自定义模式，
 * 自动根据选中的 objectClass 加载 MUST/MAY 属性表单
 *
 * 对标 phpLDAPadmin v1.2（lib/TemplateRender.php / templates/creation/*.xml）：
 * - 模板定义 RDN 时属性固定展示，不提供下拉（drawRDNChooser 渲染隐藏域）
 * - 自定义模式 RDN 候选 = 条目属性且排除 objectClass（drawRDNChooser）
 * - 模板创建时 objectClass 由模板自动设定，不作为表单项（rebuildTemplateAttrs）
 * - posix 模板属性精选，不含 shadow 杂项（posixAccount.xml / posixGroup.xml）
 * - gidNumber 默认取第一个 POSIX 组的 gid（PickList），uidNumber 自动取下一可用 ID（GetNextNumber）
 */
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Plus, Refresh, User, Folder, WarningFilled } from '@element-plus/icons-vue'
import { createEntry, getNextId } from '@/api/entry'
import { getObjectClasses, getAllowedAttributes } from '@/api/schema'
import { search } from '@/api/search'
import { useServerStore } from '@/store/server'
import { escapeDNValue } from '@/utils/dn'
import { attrLabel, isNumberAttr, isPasswordAttr } from '@/utils/attribute'
import type { ObjectClassDef } from '@/types'

// ==================== 组件属性和事件 ====================
const props = defineProps<{
  modelValue: boolean  // 控制对话框显示/隐藏
  parentDn: string     // 父级 DN（新条目将创建在此之下）
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  success: []  // 条目创建成功后触发
}>()

// ==================== 响应式状态 ====================
const formRef = ref<FormInstance>()    // 表单实例引用
const creating = ref(false)            // 创建操作加载状态
const showAddField = ref(false)        // 添加自定义属性对话框显示
const newField = ref({ name: '', value: '' })  // 待添加的自定义属性

/**
 * 预设模板 — 对标 phpLDAPadmin v1.2 内置模板
 * 参考: https://github.com/leenooks/phpLDAPadmin/tree/master/templates/creation
 */
interface TemplateDef {
  name: string             // 模板显示名称
  objectClasses: string[]  // objectClass 列表（自动按层级追加 top）
  rdn: string              // RDN 属性名（对标模板 XML 的 <rdn>，创建时固定）
  hint?: string            // 模板提示
  /** 对标 phpLDAPadmin 模板 <attributes>：默认展示的 MAY 属性（含可选默认值） */
  preferred?: Array<{ name: string; value?: string }>
  /** 当模板的 objectClasses 在当前服务器 schema 中不含 STRUCTURAL 类时
   *  （如 rfc2307bis 下 posixAccount 是 AUXILIARY），自动补充的 STRUCTURAL 类。
   *  对标 phpLDAPadmin posixAccount.xml 模板固定 inetOrgPerson + posixAccount。 */
  structuralAlt?: string
}

const templates: TemplateDef[] = [
  // ==================== 高频模板（POSIX 用户/组） ====================
  {
    // PLA posixAccount.xml: inetOrgPerson+posixAccount, rdn=cn
    // 对标 phpLDAPadmin：RDN 固定为 cn；补充 shadowAccount 支持密码老化
    name: 'POSIX 账户 (posixAccount)',
    objectClasses: ['posixAccount', 'shadowAccount', 'top'],
    structuralAlt: 'inetOrgPerson',
    rdn: 'cn',
    hint: 'Unix 系统账户',
    // 对标 PLA 模板 <attributes> 展示顺序：givenName, sn, cn(RDN), uid, homeDirectory,
    // uidNumber, gidNumber, loginShell, userPassword
    // MUST(cn,sn,uid,uidNumber,gidNumber,homeDirectory) 自动渲染，此处为 MAY 精选
    preferred: [
      { name: 'givenName' },
      { name: 'loginShell', value: '/bin/bash' },
      { name: 'userPassword' },
    ],
  },
  {
    // PLA posixGroup.xml: posixGroup, rdn=cn, attrs=cn,gidNumber,memberUid
    name: 'POSIX 组 (posixGroup)',
    objectClasses: ['posixGroup', 'top'],
    structuralAlt: 'groupOfNames',
    rdn: 'cn',
    preferred: [{ name: 'memberUid' }],
  },
  // ==================== phpLDAPadmin 内置模板 ====================
  {
    // PLA ou.xml: organizationalUnit, rdn=ou
    name: '组织单位 (Organizational Unit)',
    objectClasses: ['organizationalUnit', 'top'],
    rdn: 'ou',
    hint: '在组织下创建',
  },
  {
    // PLA 无直接对应，organization 为通用 STRUCTURAL 类
    name: '组织 (Organization)',
    objectClasses: ['organization', 'top'],
    rdn: 'o',
    hint: '在国家 (c=) 下创建',
  },
  {
    // PLA inetOrgPerson.xml: inetOrgPerson, rdn=cn, preferred: givenName,sn,mail,telephoneNumber,mobile
    name: '用户 (inetOrgPerson)',
    objectClasses: ['inetOrgPerson', 'organizationalPerson', 'person', 'top'],
    rdn: 'cn',
    hint: '在组织单位下创建',
    preferred: [
      { name: 'givenName' },
      { name: 'sn' },
      { name: 'mail' },
      { name: 'telephoneNumber' },
      { name: 'mobile' },
    ],
  },
  {
    // PLA 无直接 groupOfNames 模板，但为常用类
    name: '组 (Group of Names)',
    objectClasses: ['groupOfNames', 'top'],
    rdn: 'cn',
    preferred: [{ name: 'member' }],
  },
  {
    // PLA courierMailAccount.xml: inetOrgPerson+courierMailAccount；
    // 我们用 inetLocalMailRecipient（更通用的邮件路由辅助类）
    name: '邮件账户 (Mail Account)',
    objectClasses: ['inetOrgPerson', 'inetLocalMailRecipient', 'organizationalPerson', 'person', 'top'],
    rdn: 'cn',
    hint: '在组织单位下创建',
    preferred: [
      { name: 'givenName' },
      { name: 'sn' },
      { name: 'mail' },
      { name: 'mailRoutingAddress' },
    ],
  },
  {
    // PLA dNSDomain.xml: dnsDomain+domainRelatedObject, rdn=dc, attrs=dc,associatedDomain
    name: 'DNS 域名 (DNS Domain)',
    objectClasses: ['dnsDomain', 'domainRelatedObject', 'top'],
    rdn: 'dc',
    preferred: [{ name: 'associatedDomain' }],
  },
  {
    // PLA organizationalRole.xml: organizationalRole, rdn=cn
    name: '组织角色 (Organizational Role)',
    objectClasses: ['organizationalRole', 'top'],
    rdn: 'cn',
    preferred: [
      { name: 'telephoneNumber' },
      { name: 'facsimileTelephoneNumber' },
      { name: 'description' },
    ],
  },
  {
    // PLA simpleSecurityObject.xml: account+simpleSecurityObject, rdn=userid
    name: '安全对象 (Simple Security Object)',
    objectClasses: ['account', 'simpleSecurityObject', 'top'],
    rdn: 'uid',
    preferred: [{ name: 'userPassword' }],
  },
  {
    // PLA alias.xml: alias+extensibleObject, rdn=uid, attrs=aliasedObjectName,uid
    name: 'LDAP 别名 (Alias)',
    objectClasses: ['alias', 'extensibleObject', 'top'],
    rdn: 'uid',
    preferred: [{ name: 'aliasedObjectName' }],
  },
  {
    // PLA sambaSamAccount.xml: inetOrgPerson+sambaSAMAccount+posixAccount, rdn=cn
    name: 'Samba 账户',
    objectClasses: ['inetOrgPerson', 'posixAccount', 'sambaSamAccount', 'top'],
    rdn: 'uid',
    hint: 'Samba 账户（需 samba schema）',
    preferred: [
      { name: 'userPassword' },
      { name: 'loginShell', value: '/bin/bash' },
      { name: 'givenName' },
      { name: 'sn' },
    ],
  },
]

const objectClassList = ref<ObjectClassDef[]>([])  // 从后端获取的所有 objectClass 定义（含 type）
const availableAttrs = ref<string[]>([])           // 可添加的 MAY 属性列表

// ==================== 表单数据 ====================
interface FormField {
  name: string
  value: string
  must: boolean
  algo?: string  // 密码属性哈希算法（SSHA/SHA/MD5/PLAIN）
}

const form = reactive({
  mode: 'template',            // 创建模式：template=模板, custom=自定义
  template: templates[0].name, // 选中的模板名称
  objectClasses: [...templates[0].objectClasses], // 选中的 objectClass 列表（模板模式自动设定）
  rdnAttr: templates[0].rdn,   // RDN 属性名
  rdnValue: '',                // RDN 值（条目的唯一标识）
  fields: [] as FormField[],   // 属性表单字段（同名多个 = 多值属性）
})

/**
 * 属性短标签 — 去掉友好名称中的英文注释部分，避免表单标签挤压换行
 * 对标 phpLDAPadmin：模板表单只显示友好名称（如 "First name"）
 */
function shortLabel(name: string): string {
  return attrLabel(name).replace(/\s*\(.*\)\s*$/, '')
}

// ==================== RDN 双向同步 ====================
// RDN 值与同名属性字段保持一致 — 消除"表单填写和创建结果对不上"的根因
watch(() => form.rdnValue, (v) => {
  const f = form.fields.find((f) => f.name.toLowerCase() === form.rdnAttr.toLowerCase())
  if (f && f.value !== v) f.value = v
})
watch(() => form.rdnAttr, (attr) => {
  const f = form.fields.find((f) => f.name.toLowerCase() === attr.toLowerCase())
  if (f && f.value !== form.rdnValue) form.rdnValue = f.value
})
// 属性字段值变化时同步 RDN 值（编辑 RDN 属性字段 = 编辑 RDN 值），并做 posixAccount 默认值填充
const lastAutoHomeUid = ref('')  // 上次自动生成 homeDirectory 时使用的 uid，用于判断是否需要同步更新
watch(() => form.fields.map((f) => f.value).join('|'), () => {
  const f = form.fields.find((f) => f.name.toLowerCase() === form.rdnAttr.toLowerCase())
  if (f && f.value !== form.rdnValue) form.rdnValue = f.value
  // 对标 phpLDAPadmin autofill（posixAccount.xml）：homeDirectory 按 uid 自动生成
  // - home 为空时自动填充
  // - uid 变化时，若 home 仍是上次自动生成的值（/home/<old_uid>），则同步更新
  // - 若用户手动修改过 home，则不覆盖
  const uid = form.fields.find((x) => x.name.toLowerCase() === 'uid')
  const home = form.fields.find((x) => x.name.toLowerCase() === 'homedirectory')
  if (uid && home) {
    const uidVal = uid.value.trim()
    const homeVal = home.value.trim()
    const autoHome = `/home/${uidVal}`
    if (uidVal && !homeVal) {
      home.value = autoHome
      lastAutoHomeUid.value = uidVal
    } else if (uidVal && homeVal && lastAutoHomeUid.value && homeVal === `/home/${lastAutoHomeUid.value}`) {
      home.value = autoHome
      lastAutoHomeUid.value = uidVal
    }
  }
})

/** 实时 DN 预览 — 对标 phpLDAPadmin 创建页显示的 Container/DN */
const dnPreview = computed(() => {
  if (!form.rdnValue.trim() || !props.parentDn) return ''
  return `${form.rdnAttr}=${escapeDNValue(form.rdnValue)},${props.parentDn}`
})

/** DN 预览占位（RDN 值未填写时） */
const dnPreviewPlaceholder = computed(() => {
  if (!props.parentDn) return '—'
  return `${form.rdnAttr}=…,${props.parentDn}`
})

/** 当前选中的模板定义 */
const currentTemplate = computed(() => templates.find((t) => t.name === form.template))

/** 判断是否为密码属性（显示 helper 下拉框） */
function isPasswordField(name: string): boolean {
  return isPasswordAttr(name)
}

// ==================== 表单校验规则 ====================
// objectClass 不设为表单必填项 — 对标 phpLDAPadmin：模板模式由模板自动设定，
// 自定义模式选填，创建时服务端最终校验
const rules: FormRules = {
  rdnAttr: [{ required: true, message: '请选择 RDN 属性', trigger: 'change' }],
  rdnValue: [{ required: true, message: '请输入 RDN 值', trigger: 'blur' }],
}

// ==================== 监听对话框打开 ====================
watch(
  () => props.modelValue,
  (val) => {
    if (val) {
      loadObjectClasses()
      onTemplateChange()
    }
  },
)

/** 从后端加载所有可用的 objectClass 定义（含 type/must/may） */
async function loadObjectClasses() {
  try {
    objectClassList.value = await getObjectClasses()
    // 服务器 schema 加载后，当前模板若不可用则自动切换到第一个可用模板
    // （对标 phpLDAPadmin：只展示服务器 schema 支持的模板）
    const cur = templates.find((t) => t.name === form.template)
    if (cur && !isTemplateAvailable(cur)) {
      const firstOk = templates.find((t) => isTemplateAvailable(t))
      if (firstOk) {
        form.template = firstOk.name
        onTemplateChange()
        ElMessage.warning(`当前服务器 Schema 不支持模板「${cur.name}」，已自动切换`)
      }
    } else if (cur && form.mode === 'template') {
      // Schema 加载完成后重新解析 objectClass：
      // 对话框打开时 objectClassList 为空，resolveObjectClasses 无法判断 posixAccount 是否为 STRUCTURAL，
      // 直接返回模板原始列表（不含 structuralAlt）。若服务器使用 rfc2307bis（posixAccount=AUXILIARY），
      // 此时条目无 STRUCTURAL 类，创建会报 "no structural object class provided"。
      // 此处用已加载的 schema 重新解析，按需补充 structuralAlt（如 inetOrgPerson）。
      const resolved = resolveObjectClasses(cur)
      const changed = resolved.length !== form.objectClasses.length ||
        !resolved.every((oc, i) => oc.toLowerCase() === (form.objectClasses[i] || '').toLowerCase())
      if (changed) {
        form.objectClasses = resolved
        loadFields(cur)
      }
    }
  } catch {
    objectClassList.value = defaultObjectClassDefs()
  }
}

/** schema 加载失败时的兜底 objectClass 定义 */
function defaultObjectClassDefs(): ObjectClassDef[] {
  const names = ['inetOrgPerson', 'organizationalPerson', 'person', 'top', 'groupOfNames', 'organizationalUnit']
  return names.map((n) => ({
    oid: `1.1.${n}`,
    name: n,
    names: [n],
    description: '',
    type: 'structural',
    must: [],
    may: [],
    superior: [],
  }))
}

// ==================== 模板可用性（对标 phpLDAPadmin 按 schema 过滤模板） ====================

/** 判断某 objectClass 在服务器 schema 中的类型（structural/auxiliary/abstract） */
function ocType(name: string): string | undefined {
  return objectClassList.value.find((o) => o.name.toLowerCase() === name.toLowerCase())?.type
}

/** 找出模板中服务器 schema 不支持的 objectClass */
function missingClasses(t: TemplateDef): string[] {
  if (objectClassList.value.length === 0) return [] // schema 未知时不拦截
  const known = new Set(objectClassList.value.map((o) => o.name.toLowerCase()))
  return t.objectClasses.filter((oc) => !known.has(oc.toLowerCase()))
}

/**
 * 解析模板最终生效的 objectClass 列表 — 对标 phpLDAPadmin 确保含 STRUCTURAL 类。
 * 若模板的 objectClasses 在当前服务器 schema 中不含 STRUCTURAL 类（如 rfc2307bis 下
 * posixAccount 是 AUXILIARY），则在头部插入 structuralAlt（如 inetOrgPerson）。
 */
function resolveObjectClasses(tpl: TemplateDef): string[] {
  const ocs = tpl.objectClasses
  // schema 未加载时不做补充，保持模板原样（避免误判导致 multiple structural 错误）
  if (objectClassList.value.length === 0) return ocs
  const hasStructural = ocs.some((oc) => ocType(oc) === 'structural')
  if (hasStructural) return ocs
  if (tpl.structuralAlt && !ocs.some((oc) => oc.toLowerCase() === tpl.structuralAlt!.toLowerCase())) {
    return [tpl.structuralAlt, ...ocs]
  }
  return ocs
}

/** 模板是否可用：其全部 objectClass 都存在于服务器 schema */
function isTemplateAvailable(t: TemplateDef): boolean {
  return missingClasses(t).length === 0
}

/** 当前选中模板不可用时的提示文案 */
const unavailableHint = computed(() => {
  const cur = currentTemplate.value
  if (!cur || isTemplateAvailable(cur)) return ''
  const missing = missingClasses(cur).join(', ')
  return `当前服务器 Schema 缺少 ${missing}，该模板不可用。OpenLDAP 需在 slapd 配置中加载对应 schema（如 posixGroup 需要 nis.schema）`
})

/** 切换创建模式时重置表单 */
function onModeChange() {
  if (form.mode === 'custom') {
    form.objectClasses = []
    form.fields = []
    availableAttrs.value = []
    form.rdnAttr = 'cn'
    form.rdnValue = ''
  } else {
    onTemplateChange()
  }
}

/** 应用选中模板的 objectClass/RDN 配置并加载属性表单 — 对标 phpLDAPadmin 模板创建 */
function onTemplateChange() {
  const tpl = currentTemplate.value
  if (tpl) {
    // 动态解析 objectClass：确保含 STRUCTURAL 类（兼容 RFC 2307 / rfc2307bis）
    form.objectClasses = resolveObjectClasses(tpl)
    // 对标模板 XML 的 <rdn>：模板模式下 RDN 属性固定
    form.rdnAttr = tpl.rdn
    loadFields(tpl)
  }
}

/** 自定义模式下 objectClass 变更时重新加载 MUST/MAY 属性表单 */
function onObjectClassChange() {
  loadFields()
}

/** 根据当前选中的 objectClass 列表加载 MUST/MAY 属性字段（模板模式传入 tpl） */
async function loadFields(tpl?: TemplateDef) {
  if (form.objectClasses.length === 0) {
    form.fields = []
    availableAttrs.value = []
    return
  }
  try {
    // 防御性过滤：objectClass 由模板/表单统一维护，绝不作为属性字段出现
    // （top 的 MUST 含 objectClass，后端已过滤，此处兜底）
    const { must: rawMust, may: rawMay } = await getAllowedAttributes(form.objectClasses)
    const must = rawMust.filter((a) => a.toLowerCase() !== 'objectclass')
    const may = rawMay.filter((a) => a.toLowerCase() !== 'objectclass')
    if (!tpl) {
      const commonRDNs = ['cn', 'uid', 'ou', 'o', 'dc']
      let rdnAttr = ''
      for (const r of commonRDNs) {
        if (must.includes(r)) {
          rdnAttr = r
          break
        }
      }
      form.rdnAttr = rdnAttr || must[0] || 'cn'
    }

    // 对标 phpLDAPadmin：RDN 属性在顶部单独输入，不出现在属性表单中
    // （提交时由 handleCreate 将 RDN 值写入该属性）
    const rdnAttrLower = form.rdnAttr.toLowerCase()
    form.fields = must
      .filter((a) => a.toLowerCase() !== rdnAttrLower)
      .map((a) => ({
        name: a,
        value: '',
        must: true,
        algo: isPasswordField(a) ? 'SSHA' : undefined, // 密码属性默认 SSHA，对标 phpLDAPadmin
      }))
    form.rdnValue = ''
    lastAutoHomeUid.value = ''  // 重置 homeDirectory 自动联动跟踪

    // 对标 phpLDAPadmin 模板 <attributes>：按模板 preferred 列表展示 MAY 属性
    for (const p of tpl?.preferred || []) {
      if (may.includes(p.name) && !form.fields.some((f) => f.name === p.name)) {
        form.fields.push({
          name: p.name,
          value: p.value || '',
          must: false,
          algo: isPasswordField(p.name) ? 'SSHA' : undefined,
        })
      }
    }
    // 过滤掉已在表单中的 MAY 属性
    availableAttrs.value = may.filter((a) => !form.fields.some((f) => f.name === a))
    // 对标 phpLDAPadmin：uidNumber 自动取下一可用 ID（GetNextNumber）；
    // gidNumber — POSIX 账户模板默认取第一个 POSIX 组的 gid（PickList），
    //            POSIX 组模板取下一可用 ID（GetNextNumber）
    const isPosixAcct = !!tpl?.objectClasses.some((oc) => oc.toLowerCase() === 'posixaccount')
    autoFillIDs(isPosixAcct ? 'first-group' : 'next')
  } catch {
    form.fields = [{ name: 'cn', value: '', must: true, algo: undefined }]
    form.rdnValue = ''
  }
}

/** 自动填充 uidNumber/gidNumber
 *  uidNumber 取服务器下一可用 ID；
 *  gidNumber 按 gidMode 决定：first-group=取第一个 POSIX 组的 gid（POSIX 账户模板），next=自增（POSIX 组模板）
 */
async function autoFillIDs(gidMode: 'first-group' | 'next') {
  for (const f of form.fields) {
    const n = f.name.toLowerCase()
    if (n === 'uidnumber') await fillAutoID(f)
    if (n === 'gidnumber') {
      if (gidMode === 'first-group') await fillFirstGroupGid(f)
      else await fillAutoID(f)
    }
  }
}

/** gidNumber 默认取第一个已有 POSIX 组的 gidNumber；无组或查询失败时退回自增 */
async function fillFirstGroupGid(field: FormField) {
  try {
    const base = serverStore.currentServer()?.base_dn
    if (base) {
      const list = await search({
        base_dn: base,
        filter: '(objectClass=posixGroup)',
        scope: 'sub',
        attributes: ['cn', 'gidNumber'],
        size_limit: 1,
      })
      const gid = list[0]?.attributes.find((a) => a.name.toLowerCase() === 'gidnumber')?.values[0]
      if (gid) {
        field.value = gid
        return
      }
    }
  } catch {
    // 查询失败时退回自增
  }
  await fillAutoID(field)
}

/** 判断是否为自增 ID 属性 */
function isAutoIDField(name: string): boolean {
  const n = name.toLowerCase()
  return n === 'uidnumber' || n === 'gidnumber'
}

/** 判断是否为 loginShell 属性（下拉选择常用 shell） */
function isShellField(name: string): boolean {
  return name.toLowerCase() === 'loginshell'
}

/** phpLDAPadmin 常用登录 Shell 列表（posixAccount.xml） */
const commonShells = [
  '/bin/bash', '/bin/csh', '/bin/dash', '/bin/sh', '/bin/tsh',
  '/bin/false', '/usr/sbin/nologin', '/bin/zsh', '/sbin/nologin',
]

/** 判断是否为 POSIX 账户上下文下的 gidNumber 字段（显示文件夹图标从已有组选择） */
function isGidFieldForAccount(name: string): boolean {
  if (name.toLowerCase() !== 'gidnumber') return false
  // 仅当 objectClass 含 posixAccount 时显示（POSIX 用户需要指定所属组），
  // POSIX 组模板的 gidNumber 是自身自增编号，不需要从已有组选择
  return form.objectClasses.some((oc) => oc.toLowerCase() === 'posixaccount')
}

/** 判断是否为数值型属性（uidNumber/gidNumber/shadow*），需整数校验 */
function isNumberField(name: string): boolean {
  return isNumberAttr(name)
}

/** 数值字段的表单校验规则：必须为非负整数 */
function numberRules(field: FormField) {
  return [
    ...(field.must ? [{ required: true, message: `${shortLabel(field.name)} 为必填属性`, trigger: 'blur' as const }] : []),
    {
      pattern: /^\d+$/,
      message: `${shortLabel(field.name)} 必须为非负整数`,
      trigger: 'blur',
    },
  ]
}

/** 从服务器获取下一个可用 ID 并填充 */
async function fillAutoID(field: FormField) {
  try {
    const kind = field.name.toLowerCase() === 'uidnumber' ? 'uid' : 'gid'
    const res = await getNextId(kind as 'uid' | 'gid')
    field.value = String(res.next)
  } catch {
    // 查询失败时留空，由用户手动填写
  }
}

// ==================== 组初始成员选择（对标 phpLDAPadmin posixGroup MultiList） ====================
/** 候选用户（组成员选择器用）：DN、uid 和姓名 */
interface PickUser { dn: string; uid: string; cn: string }

const serverStore = useServerStore()               // 服务器 Store，用于获取搜索 Base DN
const showUserPick = ref(false)                    // 组成员选择器对话框显示状态
const userPickFilter = ref('')                     // 成员选择器过滤关键词
const userPickSelection = ref<PickUser[]>([])      // 选择器中已勾选的用户
const pickUsers = ref<PickUser[]>([])              // 选择器候选用户列表

/** 表单中是否存在 memberUid 字段（决定是否显示成员选择区） */
const hasMemberUidField = computed(() =>
  form.fields.some((f) => f.name.toLowerCase() === 'memberuid'),
)

/** 已选成员 uid 列表（从 memberUid 字段行提取） */
const pickedMemberUids = computed(() =>
  form.fields
    .filter((f) => f.name.toLowerCase() === 'memberuid' && f.value.trim())
    .map((f) => f.value.trim()),
)

/** 选择器候选用户过滤 */
const filteredPickUsers = computed(() => {
  const k = userPickFilter.value.trim().toLowerCase()
  if (!k) return pickUsers.value
  return pickUsers.value.filter(
    (u) => u.uid.toLowerCase().includes(k) || u.cn.toLowerCase().includes(k),
  )
})

/** 打开用户选择器，加载所有 posixAccount 用户 */
async function openUserPick() {
  const base = serverStore.currentServer()?.base_dn
  if (!base) { ElMessage.error('无法获取 Base DN'); return }
  userPickFilter.value = ''
  userPickSelection.value = []
  showUserPick.value = true
  try {
    const list = await search({
      base_dn: base,
      filter: '(&(objectClass=posixAccount)(uid=*))',
      scope: 'sub',
      size_limit: 200,
    })
    pickUsers.value = list.map((e) => ({
      dn: e.dn,
      uid: e.attributes.find((a) => a.name.toLowerCase() === 'uid')?.values[0] || '',
      cn: e.attributes.find((a) => a.name.toLowerCase() === 'cn')?.values[0] || e.dn,
    }))
  } catch {
    ElMessage.error('加载用户列表失败')
    pickUsers.value = []
  }
}

/** 确认选择：为每个 uid 追加一个 memberUid 字段行，自动去重 */
function confirmUserPick() {
  for (const u of userPickSelection.value) {
    if (!u.uid || pickedMemberUids.value.includes(u.uid)) continue
    form.fields.push({ name: 'memberUid', value: u.uid, must: false })
  }
  showUserPick.value = false
}

/** 移除一个已选成员（删除对应的 memberUid 字段行） */
function removePickedMember(uid: string) {
  const idx = form.fields.findIndex(
    (f) => f.name.toLowerCase() === 'memberuid' && f.value.trim() === uid,
  )
  if (idx >= 0) form.fields.splice(idx, 1)
}

// ==================== gidNumber 从已有 POSIX 组选择（对标 phpLDAPadmin PickList） ====================
interface GidGroup { dn: string; cn: string; gidNumber: string }

const showGidPick = ref(false)
const gidPickFilter = ref('')
const gidGroups = ref<GidGroup[]>([])
const gidTargetField = ref<FormField | null>(null)

const filteredGidGroups = computed(() => {
  const k = gidPickFilter.value.trim().toLowerCase()
  if (!k) return gidGroups.value
  return gidGroups.value.filter((g) => g.cn.toLowerCase().includes(k) || g.dn.toLowerCase().includes(k))
})

/** 打开组选择器，加载所有 posixGroup */
async function openGidGroupPick(field: FormField) {
  const base = serverStore.currentServer()?.base_dn
  if (!base) { ElMessage.error('无法获取 Base DN'); return }
  gidTargetField.value = field
  gidPickFilter.value = ''
  showGidPick.value = true
  try {
    const list = await search({
      base_dn: base,
      filter: '(objectClass=posixGroup)',
      scope: 'sub',
      attributes: ['cn', 'gidNumber'],
      size_limit: 200,
    })
    gidGroups.value = list
      .map((e) => ({
        dn: e.dn,
        cn: e.attributes.find((a) => a.name.toLowerCase() === 'cn')?.values[0] || e.dn,
        gidNumber: e.attributes.find((a) => a.name.toLowerCase() === 'gidnumber')?.values[0] || '',
      }))
      .filter((g) => g.gidNumber)
  } catch {
    ElMessage.error('加载组列表失败')
    gidGroups.value = []
  }
}

/** 选中某个组，把其 gidNumber 填入目标字段 */
function confirmGidPick(row: GidGroup) {
  if (gidTargetField.value) gidTargetField.value.value = row.gidNumber
  showGidPick.value = false
}

/**
 * RDN 属性候选（仅自定义模式使用）
 * 对标 phpLDAPadmin drawRDNChooser：列出条目属性（排除 objectClass），
 * 空表单时回退到常见命名属性
 */
const rdnCandidates = computed(() => {
  const names = new Set<string>()
  for (const f of form.fields) names.add(f.name)
  for (const a of availableAttrs.value) names.add(a)
  names.delete('objectClass')
  if (names.size === 0) ['cn', 'uid', 'ou', 'o', 'dc'].forEach((a) => names.add(a))
  return Array.from(names)
})

/** 向表单添加一个新的自定义属性字段 */
function addField() {
  if (!newField.value.name) return
  const field: FormField = {
    name: newField.value.name,
    value: '',
    must: false,
    algo: isPasswordField(newField.value.name) ? 'SSHA' : undefined,
  }
  form.fields.push(field)
  // 添加自增 ID 属性时自动分配 — 对标 phpLDAPadmin
  if (isAutoIDField(field.name)) fillAutoID(field)
  newField.value = { name: '', value: '' }
  showAddField.value = false
}

/** 从表单移除指定索引的属性字段 */
function removeField(idx: number) {
  // RDN 属性至少保留一个输入框
  const field = form.fields[idx]
  if (field && field.name.toLowerCase() === form.rdnAttr.toLowerCase()) {
    const sameAttr = form.fields.filter((f) => f.name.toLowerCase() === form.rdnAttr.toLowerCase())
    if (sameAttr.length <= 1) return
  }
  form.fields.splice(idx, 1)
}

/** 为 RDN 属性追加一个值输入框（多值属性，对标 phpLDAPadmin） */
function addValue() {
  form.fields.push({ name: form.rdnAttr, value: '', must: false })
}

/** 验证表单并调用后端 API 创建新条目 */
async function handleCreate() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    // 父 DN 缺失时拼出的 DN 会带尾逗号，LDAP 直接报 Invalid DN Syntax
    if (!props.parentDn) {
      ElMessage.error('父 DN 为空，请先在目录树中选择一个父条目')
      return
    }
    // objectClass 非必填项，但 LDAP 服务器要求条目至少含一个 objectClass —
    // 此处软提示，不作为表单校验（对标 phpLDAPadmin 由服务端最终校验）
    if (form.objectClasses.length === 0) {
      ElMessage.warning('objectClass 为空：LDAP 服务器通常要求条目至少包含一个 objectClass，创建可能失败')
    }
    creating.value = true
    try {
      // RDN 值按 RFC 4514 转义，避免 , + " \ < > ; 等字符破坏 DN 结构
      const dn = `${form.rdnAttr}=${escapeDNValue(form.rdnValue)},${props.parentDn}`

      // 同名多值属性归并到同一数组（对标 phpLDAPadmin 多值支持）
      const attributes: Record<string, string[]> = {
        objectClass: form.objectClasses,
      }
      for (const f of form.fields) {
        const v = f.value.trim()
        if (!v) continue
        if (attributes[f.name]) {
          if (!attributes[f.name].includes(v)) attributes[f.name].push(v)
        } else {
          attributes[f.name] = [v]
        }
      }
      // 确保 RDN 属性一定包含 RDN 值 — 防止"DN 有 RDN 但条目缺该属性"
      if (!attributes[form.rdnAttr]) attributes[form.rdnAttr] = []
      if (!attributes[form.rdnAttr].includes(form.rdnValue.trim())) {
        attributes[form.rdnAttr].unshift(form.rdnValue.trim())
      }

      // 收集密码属性的哈希算法
      const passwordAlgos: Record<string, string> = {}
      for (const f of form.fields) {
        if (isPasswordField(f.name) && f.algo && f.algo !== 'PLAIN') {
          passwordAlgos[f.name] = f.algo
        }
      }

      await createEntry({ dn, attributes, password_algos: passwordAlgos })
      ElMessage.success('条目创建成功')
      emit('success')
    } finally {
      creating.value = false
    }
  })
}
</script>

<style scoped lang="scss">
/* 对话框内容区：长表单时可滚动，避免溢出 */
:deep(.el-dialog__body) {
  max-height: calc(100vh - 200px);
  overflow-y: auto;
  padding-top: 12px;
}

/* 表单项统一间距 — 字段之间保持呼吸感，不再挤在一起 */
:deep(.el-form-item) {
  margin-bottom: 20px;
}

/* 标签行高与换行 */
:deep(.el-form-item__label) {
  line-height: 1.5;
  height: auto;
  min-height: 32px;
}

/* 分隔线上下留白 */
:deep(.el-divider) {
  margin: 20px 0 24px;
}

:deep(.el-divider__text) {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

/* ==================== 信息区（父 DN / DN 预览） ==================== */
.info-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px 16px;
  margin-bottom: 20px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
}

.info-row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  min-width: 0;
}

.info-label {
  flex-shrink: 0;
  width: 64px;
  font-size: var(--fs-label);
  color: var(--text-muted);
}

.info-value {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--text-secondary);
  word-break: break-all;
  line-height: 1.6;
}

.mono {
  font-family: var(--font-mono);
}

/* DN 预览占位态弱化显示 */
.is-preview {
  color: var(--text-muted);
}

/* ==================== 模板提示 / Schema 警告 ==================== */
.template-hint {
  width: 100%;
  margin-top: 4px;
  font-size: var(--fs-caption);
  color: var(--text-muted);
  line-height: 1.5;
}

.schema-warn {
  width: 100%;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--warning, #e6a23c);
  display: flex;
  align-items: flex-start;
  gap: 4px;
}

/* ==================== RDN 固定展示（模板模式） ==================== */
.rdn-fixed {
  display: flex;
  align-items: center;
  gap: 8px;
  line-height: 1.8;

  .rdn-fixed-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--accent-primary);
  }

  .rdn-fixed-hint {
    font-size: var(--fs-caption);
    color: var(--text-muted);
  }
}

/* RDN 属性标记 */
.rdn-tag {
  margin-left: 6px;
  transform: scale(0.85);
  transform-origin: left center;
}

/* ==================== 属性字段行 ==================== */
.field-label-text {
  /* 允许标签换行，避免长名称挤压 */
  display: inline;
  word-break: break-all;
}

.field-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;

  :deep(.el-form-item) {
    flex: 1;
    min-width: 0;
    margin-bottom: 18px;
  }

  /* 移除按钮固定列宽，与输入框垂直居中对齐，不挤压表单项 */
  .field-remove {
    flex-shrink: 0;
    width: 48px;
    margin-top: 4px;
  }
}

.field-input-row {
  display: flex;
  gap: 8px;
  width: 100%;
  align-items: center;

  .shell-select {
    flex: 1;
    min-width: 0;
  }

  .pwd-algo {
    width: 110px;
    flex-shrink: 0;
  }
}

/* ==================== 底部操作按钮 ==================== */
.form-actions {
  display: flex;
  gap: 12px;
  padding-top: 4px;
  padding-bottom: 8px;
}

/* ==================== 组初始成员选择区 ==================== */
.member-section {
  margin: 4px 0 8px;
  padding: 14px 16px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);

  .ms-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .ms-title {
    font-size: var(--fs-label);
    font-weight: 600;
    color: var(--text-secondary);
  }

  .ms-chips {
    margin-top: 12px;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .ms-empty {
    margin-top: 12px;
    font-size: var(--fs-caption);
    color: var(--text-muted);
  }
}

/* 子对话框 body 内边距 */
.dlg-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* picker 提示文字 */
.picker-hint {
  font-size: var(--fs-caption);
  color: var(--text-muted);
}
</style>
