<template>
  <div class="entry-view">
    <!-- Empty -->
    <div v-if="!entryStore.selectedDN" class="empty">
      <div class="empty-card">
        <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
        </svg>
        <h3>未选中条目</h3>
        <p>点击目录树中的节点以查看和编辑其属性。</p>
      </div>
    </div>

    <template v-else>
      <!-- Header -->
      <div class="entry-head">
        <div class="eh-main">
          <el-breadcrumb separator="›">
            <el-breadcrumb-item
              v-for="(part, idx) in dnParts"
              :key="idx"
              :to="idx < dnParts.length - 1 ? { path: '/entry', query: { dn: part.dn } } : undefined"
            >{{ part.value }}</el-breadcrumb-item>
          </el-breadcrumb>

          <div class="dn-row">
            <span class="dn-kicker">可分辨名称</span>
            <code class="dn-value">{{ entryStore.selectedDN }}</code>
            <button class="copy-btn" @click="copyDN" title="复制 DN">
              <el-icon :size="12"><CopyDocument /></el-icon>
            </button>
          </div>

          <div class="oc-row">
            <span class="oc-kicker">objectClass</span>
            <span v-for="oc in entry?.object_classes || []" :key="oc" class="oc-chip">{{ oc }}</span>
          </div>
        </div>

        <div class="eh-tools">
          <button class="tb primary" @click="showCreate = true">
            <el-icon :size="14"><Plus /></el-icon>新建子条目
          </button>
          <button class="tb" @click="handleCopy"><el-icon :size="14"><CopyDocument /></el-icon>复制</button>
          <button class="tb" @click="showMove = true"><el-icon :size="14"><Rank /></el-icon>移动</button>
          <button v-if="canJoinGroup" class="tb" @click="openJoinGroup">
            <el-icon :size="14"><UserFilled /></el-icon>加入组
          </button>
          <div class="tb-sep" />
          <button class="tb" @click="handleExport"><el-icon :size="14"><Download /></el-icon>导出 LDIF</button>
          <button class="tb danger" @click="handleDelete"><el-icon :size="14"><Delete /></el-icon>删除</button>
        </div>
      </div>

      <!-- posix 用户所属组列表 — 对标 phpLDAPadmin Groups 区块 -->
      <div v-if="isPosixUser" class="groups-area">
        <div class="attr-head">
          <div class="ah-left">
            <span class="ah-bar" />
            <span class="ah-title">所属组</span>
            <span class="ah-count font-mono">{{ userGroups.length }}</span>
          </div>
        </div>
        <div v-if="userGroups.length" class="groups-list">
          <div
            v-for="g in userGroups"
            :key="g.dn"
            class="group-chip"
            @click="entryStore.selectEntry(g.dn)"
          >
            <el-icon :size="13"><UserFilled /></el-icon>
            <span class="gc-cn">{{ g.cn }}</span>
            <span v-if="g.gidNumber" class="gc-gid font-mono">gid={{ g.gidNumber }}</span>
          </div>
        </div>
        <div v-else class="groups-empty">该用户尚未加入任何组，点击上方"加入组"添加</div>
      </div>

      <!-- Attributes list -->
      <div class="attr-area">
        <div class="attr-head">
          <div class="ah-left">
            <span class="ah-bar" />
            <span class="ah-title">属性列表</span>
            <span class="ah-count font-mono">{{ entry?.attributes?.length || 0 }}</span>
          </div>
          <button class="ah-add" @click="showAddAttr = true">
            <el-icon :size="13"><Plus /></el-icon>添加属性
          </button>
        </div>

        <div class="attr-list">
          <div v-if="entry?.attributes?.length" class="attr-grid">
            <div
              v-for="attr in entry.attributes"
              :key="attr.name"
              class="attr-card"
            >
              <div class="ac-left">
                <div class="ac-icon" :class="getIconClass(attr.name)">
                  {{ attr.name[0]?.toUpperCase() }}
                </div>
                <div class="ac-meta">
                  <span class="ac-friendly">{{ attrLabel(attr.name) }}</span>
                  <code class="ac-name font-mono">{{ attr.name }}</code>
                  <span v-if="attr.values.length > 1" class="ac-multi">
                    {{ attr.values.length }} 个值
                  </span>
                </div>
              </div>

              <div class="ac-values">
                <template v-if="isBinary(attr.name)">
                  <!-- jpegPhoto / thumbnailPhoto 内嵌缩略图预览 — 对标 phpLDAPadmin view_jpeg_photo.php -->
                  <template v-if="isImageBinary(attr.name)">
                    <div v-if="binaryCache[attr.name]" class="bin-preview">
                      <img :src="`data:image/jpeg;base64,${binaryCache[attr.name]}`" alt="photo" />
                      <span class="bin-size font-mono">{{ binarySizes[attr.name] }} bytes</span>
                    </div>
                    <div v-else-if="binaryLoading[attr.name]" class="bin-loading">
                      <el-icon :size="14" class="spin"><Loading /></el-icon>加载中…
                    </div>
                    <div v-else class="bin-missing">未设置</div>
                    <button v-if="binaryCache[attr.name]" class="val-btn" @click="downloadBinary(attr.name)" title="下载原始文件">
                      <el-icon :size="12"><Download /></el-icon>下载
                    </button>
                  </template>
                  <!-- 其他二进制属性 — 显示大小 + 下载按钮 — 对标 phpLDAPadmin download_binary_attr.php -->
                  <template v-else>
                    <div v-if="binarySizes[attr.name]" class="bin-generic">
                      <el-icon :size="14"><Document /></el-icon>
                      <span class="bin-size font-mono">{{ binarySizes[attr.name] }} bytes</span>
                    </div>
                    <div v-else class="bin-missing">
                      <el-tag size="small" class="tag-bin"><el-icon :size="12"><Picture /></el-icon>二进制</el-tag>
                    </div>
                    <button class="val-btn" @click="fetchAndShowBinary(attr.name)" title="获取二进制值">
                      {{ binarySizes[attr.name] ? '刷新' : '加载' }}
                    </button>
                    <button v-if="binarySizes[attr.name]" class="val-btn" @click="downloadBinary(attr.name)" title="下载">
                      <el-icon :size="12"><Download /></el-icon>下载
                    </button>
                  </template>
                </template>

                <template v-else-if="isPassword(attr.name)">
                  <el-tag size="small" class="tag-pwd">
                    <el-icon :size="12"><Lock /></el-icon>
                    {{ (attr.values[0] || '').slice(0, 10) }}…
                  </el-tag>
                  <button class="val-btn" @click="editPassword()">修改</button>
                </template>

                <template v-else>
                  <div class="val-chips">
                    <div
                      v-for="(val, i) in attr.values"
                      :key="i"
                      class="val-chip"
                      :class="{ 'val-chip-member': isMemberAttr(attr.name), 'val-chip-resolved': isMemberAttr(attr.name) || attr.name.toLowerCase() === 'gidnumber' }"
                      :title="val"
                    >
                      <!-- 成员 uid 解析为 "cn (uid)"，gidNumber 解析为 "组名 (gid)" — 对标 phpLDAPadmin -->
                      <template v-if="isMemberAttr(attr.name) || attr.name.toLowerCase() === 'gidnumber'">
                        <template v-if="resolvedName(attr.name, val)">
                          <span class="vc-name">{{ resolvedName(attr.name, val) }}</span>
                          <span class="vc-raw font-mono">({{ val }})</span>
                        </template>
                        <code v-else>{{ val }}</code>
                      </template>
                      <code v-else>{{ val }}</code>
                      <el-icon
                        v-if="isMemberAttr(attr.name)"
                        class="chip-remove"
                        @click="removeMemberValue(attr.name, val)"
                        title="移除此成员"
                      ><Close /></el-icon>
                    </div>
                  </div>
                </template>
              </div>

              <div class="ac-actions">
                <!-- 二进制属性上传 — 对标 phpLDAPadmin 的 jpegPhoto / userCertificate 文件上传 -->
                <button
                  v-if="isBinary(attr.name)"
                  class="aa-btn"
                  @click="openBinaryUpload(attr)"
                  title="上传/替换二进制值"
                >
                  <el-icon :size="14"><Upload /></el-icon>
                </button>
                <!-- Compare — LDAP Compare 操作（对标 phpLDAPadmin compare.php） -->
                <button
                  v-if="!isBinary(attr.name) && !isPassword(attr.name)"
                  class="aa-btn"
                  @click="openCompare(attr)"
                  title="比较属性值"
                >
                  <el-icon :size="14"><Tools /></el-icon>
                </button>
                <!-- 成员属性 helper — 对标 phpLDAPadmin 的 add member -->
                <button
                  v-if="isMemberAttr(attr.name)"
                  class="aa-btn"
                  @click="openMemberPicker(attr)"
                  title="添加成员"
                >
                  <el-icon :size="14"><UserFilled /></el-icon>
                </button>
                <button v-if="!isPassword(attr.name)" class="aa-btn" @click="editAttr(attr)" title="编辑">
                  <el-icon :size="14"><Edit /></el-icon>
                </button>
                <button class="aa-btn danger" @click="deleteAttr(attr)" title="删除">
                  <el-icon :size="14"><Delete /></el-icon>
                </button>
              </div>
            </div>
          </div>

          <div v-else class="attr-empty">
            <p>暂无属性</p>
            <button class="add-attr-sm" @click="showAddAttr = true">
              <el-icon :size="12"><Plus /></el-icon>添加第一个属性
            </button>
          </div>
        </div>
      </div>
    </template>

    <!-- Dialogs (structure kept, styled via global + scoped) -->
    <CreateEntryDialog
      v-model="showCreate"
      :parent-dn="entryStore.selectedDN"
      @success="onEntryChanged"
    />

    <el-dialog v-model="showMove" title="移动条目" width="480px">
      <div class="dlg-body">
        <div class="dlg-row">
          <span class="dlg-label">当前</span>
          <code class="dlg-val font-mono">{{ entryStore.selectedDN }}</code>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">新父级 DN</span>
          <div class="dlg-input-row">
            <el-input v-model="moveForm.newParent" placeholder="ou=newparent,dc=example,dc=com" />
            <button class="pick-btn" @click="showMoveChooser = true" title="从树中选择">
              <el-icon :size="14"><Search /></el-icon>
            </button>
          </div>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">新 RDN（可选）</span>
          <el-input v-model="moveForm.newRDN" placeholder="留空则保持不变" />
        </div>
      </div>
      <template #footer>
        <el-button @click="showMove = false">取消</el-button>
        <el-button type="primary" :loading="moving" @click="doMove">移动</el-button>
      </template>
    </el-dialog>

    <!-- EntryChooser — 用于 move 选择父级 -->
    <EntryChooserDialog
      v-model="showMoveChooser"
      title="选择新父级"
      @confirm="dn => { moveForm.newParent = dn }"
    />

    <!-- Copy 独立对话框 — 对标 phpLDAPadmin copy_form.php -->
    <el-dialog v-model="showCopy" title="复制条目" width="560px">
      <div class="dlg-body">
        <div class="dlg-row">
          <span class="dlg-label">源条目</span>
          <code class="dlg-val font-mono">{{ entryStore.selectedDN }}</code>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">目标服务器</span>
          <el-select v-model="copyForm.targetServerId" placeholder="当前服务器（跨服务器复制请选择）" clearable style="width: 100%">
            <el-option
              v-for="s in serverList"
              :key="s.id"
              :label="s.name"
              :value="s.id"
            />
          </el-select>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">目标位置（新父级 DN）</span>
          <div class="dlg-input-row">
            <el-input v-model="copyForm.destParent" placeholder="ou=target,dc=example,dc=com" />
            <button class="pick-btn" @click="showCopyChooser = true" title="从树中选择">
              <el-icon :size="14"><Search /></el-icon>
            </button>
          </div>
          <div v-if="copyForm.targetServerId" class="dlg-hint">目标服务器可能 baseDN 不同，请确认目标位置 DN 格式正确</div>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">新 RDN（可选）</span>
          <el-input v-model="copyForm.newRDN" placeholder="留空则保持源 RDN 不变" />
        </div>
        <div class="dlg-row">
          <el-checkbox v-model="copyForm.recursive">包含所有子条目（递归）</el-checkbox>
        </div>
      </div>
      <template #footer>
        <el-button @click="showCopy = false">取消</el-button>
        <el-button type="primary" :loading="copying" @click="doCopy">复制</el-button>
      </template>
    </el-dialog>

    <!-- EntryChooser — 用于 copy 选目标位置 -->
    <EntryChooserDialog
      v-model="showCopyChooser"
      title="选择目标父级"
      @confirm="dn => { copyForm.destParent = dn }"
    />

    <el-dialog v-model="showEditAttr" title="编辑属性" width="560px">
      <div class="dlg-body">
        <div class="dlg-row">
          <span class="dlg-label">属性</span>
          <code class="dlg-val font-mono">{{ editForm.name }}</code>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">值</span>
          <div class="val-edit-list">
            <div v-for="(_, i) in editForm.values" :key="i" class="val-edit-row">
              <el-input v-model="editForm.values[i]" type="textarea" :rows="2" />
              <button
                v-if="editForm.values.length > 1"
                class="val-edit-del"
                @click="editForm.values.splice(i, 1)"
              >
                <el-icon :size="16"><Delete /></el-icon>
              </button>
            </div>
          </div>
          <button class="add-value-btn" @click="editForm.values.push('')">
            <el-icon :size="12"><Plus /></el-icon> 添加值
          </button>
        </div>
      </div>
      <template #footer>
        <el-button @click="showEditAttr = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveAttr">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showAddAttr" title="添加属性" width="480px">
      <div class="dlg-body">
        <div class="dlg-row">
          <span class="dlg-label">属性名</span>
          <el-select v-model="addForm.name" filterable allow-create style="width: 100%">
            <el-option v-for="a in allowedAttrs" :key="a" :label="a" :value="a" />
          </el-select>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">值</span>
          <el-input v-model="addForm.value" type="textarea" :rows="4" placeholder="每行一个值以添加多个" />
        </div>
      </div>
      <template #footer>
        <el-button @click="showAddAttr = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveAddAttr">添加</el-button>
      </template>
    </el-dialog>

    <!-- 成员选择器 — 对标 phpLDAPadmin add member helper -->
    <el-dialog v-model="showMemberPicker" :title="`添加成员到 ${memberPickForm.attr}`" width="680px">
      <div class="dlg-body">
        <el-input v-model="memberFilter" placeholder="输入 uid 或姓名过滤" clearable />
        <el-table
          :data="filteredCandidates"
          max-height="360"
          @selection-change="(rows: CandidateUser[]) => memberSelection = rows"
        >
          <el-table-column type="selection" width="45" />
          <el-table-column prop="uid" label="UID" width="140" />
          <el-table-column prop="cn" label="姓名" width="160" />
          <el-table-column prop="dn" label="DN" show-overflow-tooltip />
        </el-table>
        <div class="picker-hint">
          {{ memberPickForm.attr.toLowerCase() === 'memberuid'
            ? '将添加所选用户的 uid 值（posixGroup 使用 memberUid 而非 DN）'
            : '将添加所选用户的完整 DN' }}
        </div>
      </div>
      <template #footer>
        <el-button @click="showMemberPicker = false">取消</el-button>
        <el-button type="primary" :loading="saving" :disabled="!memberSelection.length" @click="confirmAddMembers">
          添加所选（{{ memberSelection.length }}）
        </el-button>
      </template>
    </el-dialog>

    <!-- 加入组 — 对标 phpLDAPadmin add to group helper -->
    <el-dialog v-model="showJoinGroup" title="加入组" width="640px">
      <div class="dlg-body">
        <el-input v-model="groupFilter" placeholder="输入组名过滤" clearable />
        <el-table
          :data="filteredGroups"
          max-height="360"
          @selection-change="(rows: CandidateGroup[]) => groupSelection = rows"
        >
          <el-table-column type="selection" width="45" />
          <el-table-column prop="cn" label="组名" width="180" />
          <el-table-column prop="gidNumber" label="GID" width="100" />
          <el-table-column prop="dn" label="DN" show-overflow-tooltip />
        </el-table>
        <div class="picker-hint">
          {{ joinGroupHint }}
        </div>
      </div>
      <template #footer>
        <el-button @click="showJoinGroup = false">取消</el-button>
        <el-button type="primary" :loading="saving" :disabled="!groupSelection.length" @click="confirmJoinGroups">
          加入所选（{{ groupSelection.length }}）
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEditPwd" title="修改密码" width="420px">
      <div class="dlg-body">
        <div class="dlg-row">
          <span class="dlg-label">新密码</span>
          <el-input v-model="pwdForm.password" type="password" show-password />
        </div>
        <div class="dlg-row">
          <span class="dlg-label">哈希算法</span>
          <el-select v-model="pwdForm.algo" style="width: 100%">
            <el-option label="SSHA" value="SSHA" />
            <el-option label="SHA" value="SHA" />
            <el-option label="MD5" value="MD5" />
            <el-option label="CRYPT" value="CRYPT" />
            <el-option label="明文" value="plain" />
          </el-select>
        </div>
      </div>
      <template #footer>
        <el-button @click="showEditPwd = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="savePassword">保存</el-button>
      </template>
    </el-dialog>

    <!-- LDAP Compare 对话框 — 对标 phpLDAPadmin compare.php -->
    <el-dialog v-model="showCompare" title="Compare 比较属性值" width="440px">
      <div class="dlg-body">
        <div class="dlg-row">
          <span class="dlg-label">条目 DN</span>
          <code class="dlg-val font-mono">{{ entryStore.selectedDN }}</code>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">属性</span>
          <el-input v-model="compareForm.attr" disabled />
        </div>
        <div class="dlg-row">
          <span class="dlg-label">待比较值</span>
          <el-input v-model="compareForm.value" placeholder="输入要与服务器上该属性比较的值" />
        </div>
        <div v-if="compareResult !== null" class="compare-result" :class="compareResult ? 'match' : 'nomatch'">
          <el-icon :size="16"><component :is="compareResult ? Check : CloseBold" /></el-icon>
          {{ compareResult ? '✅ 服务器上该属性包含此值' : '❌ 服务器上该属性不包含此值' }}
        </div>
      </div>
      <template #footer>
        <el-button @click="showCompare = false">关闭</el-button>
        <el-button type="primary" :loading="compareWorking" @click="doCompare">比较</el-button>
      </template>
    </el-dialog>

    <!-- 二进制属性上传 — 对标 phpLDAPadmin 的 jpegPhoto / userCertificate 文件上传表单 -->
    <el-dialog v-model="showBinaryUpload" title="上传二进制属性" width="460px">
      <div class="dlg-body">
        <div class="dlg-row">
          <span class="dlg-label">条目 DN</span>
          <code class="dlg-val font-mono">{{ entryStore.selectedDN }}</code>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">属性</span>
          <el-input :model-value="binaryUploadForm.attr" disabled />
        </div>
        <div class="dlg-row">
          <span class="dlg-label">文件</span>
          <div class="upload-area">
            <el-upload
              :show-file-list="false"
              :auto-upload="false"
              :limit="1"
              :on-change="onBinaryFileChange"
              drag
            >
              <el-icon class="el-icon--upload"><Upload /></el-icon>
              <div class="el-upload__text">拖拽文件到此，或 <em>点击选择</em></div>
            </el-upload>
            <div v-if="binaryUploadForm.fileName" class="upload-preview">
              <el-icon :size="14"><Document /></el-icon>
              <span>{{ binaryUploadForm.fileName }}</span>
              <span class="font-mono">{{ binaryUploadForm.size }} bytes</span>
            </div>
            <div class="dlg-hint" v-if="isImageBinary(binaryUploadForm.attr) && binaryUploadForm.base64">
              图片预览：
              <img :src="`data:image/jpeg;base64,${binaryUploadForm.base64}`" style="width: 80px; height: 80px; object-fit: cover; border-radius: 4px; margin-top: 6px;" />
            </div>
          </div>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">操作</span>
          <el-select v-model="binaryUploadForm.op" style="width: 100%">
            <el-option label="replace — 替换当前值（覆盖）" value="replace" />
            <el-option label="add — 追加（多值属性）" value="add" />
          </el-select>
        </div>
      </div>
      <template #footer>
        <el-button @click="showBinaryUpload = false">取消</el-button>
        <el-button type="primary" :loading="binaryUploading" :disabled="!binaryUploadForm.base64" @click="doBinaryUpload">
          上传
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * 条目详情视图组件 — 展示和编辑单个 LDAP 条目的完整信息
 * 功能包括：条目导航面包屑、DN 显示、objectClass 展示、属性值增删改、
 * 条目复制、移动、删除、LDIF 导出、密码修改等
 */
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, CopyDocument, Rank, Delete, Download, Edit, Lock, Picture, UserFilled, Close,
  Loading, Document, Search, Tools, Check, CloseBold, Upload,
} from '@element-plus/icons-vue'
import { useEntryStore } from '@/store/entry'
import { useServerStore } from '@/store/server'
import {
  getEntry, modifyEntry, deleteEntry as apiDelete,
  moveEntry, copyEntry, getBinaryAttr, downloadBinaryAttr, listDescendants, compareEntry,
} from '@/api/entry'
import { search } from '@/api/search'
import { exportLdif } from '@/api/ldif'
import { getAllowedAttributes } from '@/api/schema'
import { getServers } from '@/api/server'
import { parseDN } from '@/utils/dn'
import { attrLabel, isPasswordAttr, isMemberAttr as isMemberAttrShared } from '@/utils/attribute'
import CreateEntryDialog from '@/components/entry/CreateEntryDialog.vue'
import EntryChooserDialog from '@/components/entry/EntryChooserDialog.vue'
import type { LdapAttribute, LdapEntry } from '@/types'

// ==================== 路由和 Store 初始化 ====================
const route = useRoute()
const router = useRouter()
const entryStore = useEntryStore()

// ==================== 数据状态 ====================
const entry = ref<LdapEntry | null>(null)     // 当前条目的详细数据
const allowedAttrs = ref<string[]>([])         // 可添加的属性名列表

// ==================== 对话框显示状态 ====================
const showCreate = ref(false)    // 新建子条目对话框
const showMove = ref(false)      // 移动条目对话框
const showCopy = ref(false)      // 复制条目对话框（对标 phpLDAPadmin copy_form.php）
const showMoveChooser = ref(false)   // move 专用 EntryChooser
const showCopyChooser = ref(false)   // copy 专用 EntryChooser
const showEditAttr = ref(false)  // 编辑属性对话框
const showAddAttr = ref(false)   // 添加属性对话框
const showEditPwd = ref(false)   // 修改密码对话框
const showCompare = ref(false)   // LDAP Compare 对话框（对标 phpLDAPadmin compare.php）
const compareWorking = ref(false)
const compareResult = ref<boolean | null>(null)
const compareForm = ref({ attr: '', value: '' })

// ==================== 二进制属性上传 — 对标 phpLDAPadmin 的文件上传表单 ====================
const showBinaryUpload = ref(false)
const binaryUploading = ref(false)
const binaryUploadForm = ref({
  attr: '',
  op: 'replace' as 'replace' | 'add',
  fileName: '',
  size: 0,
  base64: '',
})

function openBinaryUpload(attr: { name: string }) {
  binaryUploadForm.value = { attr: attr.name, op: 'replace', fileName: '', size: 0, base64: '' }
  showBinaryUpload.value = true
}

function onBinaryFileChange(file: any) {
  if (!file?.raw) return
  const reader = new FileReader()
  reader.onload = () => {
    // reader.result 是 data:xxx;base64,xxx 格式 — 提取后面的 base64 部分
    const dataUrl = reader.result as string
    const commaIdx = dataUrl.indexOf(',')
    binaryUploadForm.value.base64 = commaIdx >= 0 ? dataUrl.slice(commaIdx + 1) : ''
    binaryUploadForm.value.fileName = file.name
    binaryUploadForm.value.size = file.raw.size
  }
  reader.readAsDataURL(file.raw)
}

async function doBinaryUpload() {
  if (!entryStore.selectedDN || !binaryUploadForm.value.base64) return
  binaryUploading.value = true
  try {
    await modifyEntry(entryStore.selectedDN, [{
      op: binaryUploadForm.value.op,
      attr: binaryUploadForm.value.attr,
      values: [binaryUploadForm.value.base64],
    }], {})
    ElMessage.success(`二进制属性 ${binaryUploadForm.value.attr} 已更新`)
    showBinaryUpload.value = false
    // 重新加载条目 + 刷新二进制缓存
    await loadEntry(entryStore.selectedDN)
  } catch (e: any) {
    ElMessage.error(e?.message || '上传失败')
  } finally { binaryUploading.value = false }
}

// ==================== 操作加载状态 ====================
const moving = ref(false)   // 移动操作加载中
const copying = ref(false)  // 复制操作加载中
const saving = ref(false)   // 保存操作加载中

// ==================== 表单数据 ====================
const moveForm  = ref({ newParent: '', newRDN: '' })                          // 移动表单
const copyForm  = ref({ destParent: '', newRDN: '', recursive: true, targetServerId: null as number | null })        // 复制表单（对标 phpLDAPadmin copy_form.php）
const editForm  = ref<{ name: string; values: string[] }>({ name: '', values: [] })  // 编辑属性表单
const addForm   = ref({ name: '', value: '' })                                // 添加属性表单
const pwdForm   = ref({ password: '', algo: 'SSHA' })                         // 密码表单

// ==================== 二进制属性缓存（对标 phpLDAPadmin view_jpeg_photo / download_binary_attr） ====================
const binaryCache   = ref<Record<string, string>>({})     // attr name → base64
const binaryLoading = ref<Record<string, boolean>>({})
const binarySizes   = ref<Record<string, number>>({})

/** 判断是否为图片类二进制属性（直接内嵌预览） */
function isImageBinary(n: string) {
  return ['jpegphoto', 'thumbnailphoto'].includes(n.toLowerCase())
}

/** 加载指定二进制属性的值到缓存 */
async function fetchAndShowBinary(attrName: string) {
  const dn = entryStore.selectedDN
  if (!dn) return
  binaryLoading.value[attrName] = true
  try {
    const res = await getBinaryAttr(dn, attrName)
    binaryCache.value[attrName] = res.base64
    binarySizes.value[attrName] = res.size
  } catch (err: any) {
    ElMessage.error(err?.message || `加载 ${attrName} 失败`)
  } finally {
    binaryLoading.value[attrName] = false
  }
}

/** 下载二进制属性原始文件 */
function downloadBinary(attrName: string) {
  const dn = entryStore.selectedDN
  if (!dn) return
  downloadBinaryAttr(dn, attrName)
}

/** 自动预加载条目的图片类二进制属性（进入条目时自动显示头像） */
async function preloadImageBinaries() {
  if (!entry.value) return
  for (const attr of entry.value.attributes || []) {
    if (isImageBinary(attr.name) && attr.values?.length) {
      await fetchAndShowBinary(attr.name)
    }
  }
}

/** 解析选中 DN 为面包屑导航数据 */
const dnParts = computed(() => {
  const dn = entryStore.selectedDN
  if (!dn) return []
  const parts = parseDN(dn)
  const result: { dn: string; value: string }[] = []
  let current = ''
  for (let i = parts.length - 1; i >= 0; i--) {
    current = current ? `${parts[i].attr}=${parts[i].value},${current}` : `${parts[i].attr}=${parts[i].value}`
    result.unshift({ dn: current, value: parts[i].value })
  }
  return result
})

// ==================== 监听选中条目变化 ====================
watch(
  () => entryStore.selectedDN,
  dn => { if (dn) loadEntry(dn) },
  { immediate: true },
)

/** 从后端加载条目的详细数据 */
async function loadEntry(dn: string) {
  try {
    const data = await getEntry(dn)
    entry.value = data
    entryStore.setEntry(data)
    // 根据 objectClasses 获取可添加的属性列表
    if (data.object_classes.length) {
      const { may } = await getAllowedAttributes(data.object_classes)
      allowedAttrs.value = may
    }
    // posix 用户加载所属组列表
    await loadUserGroups()
    // 解析成员 uid / gidNumber 为友好名称
    await resolveNames()
    // 自动预加载图片类二进制属性（头像等） — 对标 phpLDAPadmin view_jpeg_photo
    binaryCache.value = {}; binarySizes.value = {}; binaryLoading.value = {}
    preloadImageBinaries()
  } catch { entry.value = null }
}

/** 判断属性是否为二进制类型 */
function isBinary(n: string) {
  return ['jpegphoto', 'usercertificate', 'thumbnailphoto', 'objectsid', 'objectguid']
    .includes(n.toLowerCase())
}
/** 判断属性是否为密码类型 */
function isPassword(n: string) {
  return isPasswordAttr(n)
}
/** 根据属性名返回图标样式类 */
function getIconClass(name: string) {
  const n = name.toLowerCase()
  if (n.includes('password') || n.includes('secret')) return 'ic-danger'
  if (n.includes('mail') || n.includes('email'))       return 'ic-success'
  if (n.includes('photo') || n.includes('cert'))      return 'ic-purple'
  return 'ic-primary'
}

/** 复制当前 DN 到剪贴板 */
function copyDN() {
  if (entryStore.selectedDN) {
    navigator.clipboard.writeText(entryStore.selectedDN)
    ElMessage.success('DN 已复制')
  }
}

// ==================== 属性编辑操作 ====================
/** 打开属性编辑对话框，预填当前值 */
function editAttr(row: LdapAttribute) {
  editForm.value = { name: row.name, values: [...row.values] }
  showEditAttr.value = true
}
/** 保存编辑后的属性值 */
async function saveAttr() {
  if (!entryStore.selectedDN) return
  saving.value = true
  try {
    await modifyEntry(entryStore.selectedDN, [
      { op: 'replace', attr: editForm.value.name, values: editForm.value.values.filter(Boolean) },
    ])
    ElMessage.success('已更新')
    showEditAttr.value = false
    loadEntry(entryStore.selectedDN)
  } finally { saving.value = false }
}
/** 删除指定属性（需用户确认） */
async function deleteAttr(row: LdapAttribute) {
  if (!entryStore.selectedDN) return
  await ElMessageBox.confirm(`确定删除属性 "${row.name}" 吗？`, '确认', { type: 'warning' })
  try {
    await modifyEntry(entryStore.selectedDN, [{ op: 'delete', attr: row.name }])
    ElMessage.success('已删除')
    loadEntry(entryStore.selectedDN)
  } catch { /* ignore */ }
}
/** 添加新属性到当前条目 */
async function saveAddAttr() {
  if (!entryStore.selectedDN || !addForm.value.name) return
  saving.value = true
  try {
    const values = addForm.value.value.split('\n').filter(Boolean)
    await modifyEntry(entryStore.selectedDN, [{ op: 'add', attr: addForm.value.name, values }])
    ElMessage.success('已添加')
    showAddAttr.value = false
    addForm.value = { name: '', value: '' }
    loadEntry(entryStore.selectedDN)
  } finally { saving.value = false }
}

// ==================== LDAP Compare — 对标 phpLDAPadmin compare.php ====================
function openCompare(attr: { name: string; values: string[] }) {
  compareForm.value = { attr: attr.name, value: attr.values[0] || '' }
  compareResult.value = null
  showCompare.value = true
}
async function doCompare() {
  if (!entryStore.selectedDN || !compareForm.value.attr || !compareForm.value.value) {
    ElMessage.warning('请填写完整属性和值')
    return
  }
  compareWorking.value = true
  compareResult.value = null
  try {
    const res = await compareEntry(entryStore.selectedDN, compareForm.value.attr, compareForm.value.value)
    compareResult.value = res.matched
  } catch (e: any) {
    ElMessage.error(e?.message || 'Compare 失败')
  } finally { compareWorking.value = false }
}

// ==================== 密码修改 ====================
/** 打开修改密码对话框 */
function editPassword() { pwdForm.value = { password: '', algo: 'SSHA' }; showEditPwd.value = true }
/** 对标 phpLDAPadmin password_checker.php — 前端密码强度预校验 */
function checkPasswordStrength(pwd: string): { level: 'weak' | 'medium' | 'strong'; tips: string[] } {
  const tips: string[] = []
  let score = 0
  if (pwd.length < 8) tips.push('密码长度不足 8 位'); else score += 1
  if (/[a-z]/.test(pwd)) score += 1; else tips.push('建议包含小写字母')
  if (/[A-Z]/.test(pwd)) score += 1; else tips.push('建议包含大写字母')
  if (/\d/.test(pwd)) score += 1; else tips.push('建议包含数字')
  if (/[^A-Za-z0-9]/.test(pwd)) score += 1; else tips.push('建议包含特殊字符')
  const level: 'weak' | 'medium' | 'strong' = score <= 2 ? 'weak' : score <= 3 ? 'medium' : 'strong'
  return { level, tips }
}
/** 保存新密码，由后端按选定算法哈希（对标 phpLDAPadmin password helper） */
async function savePassword() {
  if (!entryStore.selectedDN) return
  const pwd = pwdForm.value.password
  if (!pwd) { ElMessage.warning('密码不能为空'); return }
  // 密码强度预校验（对标 phpLDAPadmin password_checker.php）
  const { level, tips } = checkPasswordStrength(pwd)
  if (level === 'weak') {
    const ret = await ElMessageBox.confirm(
      `当前密码强度较弱，${tips.join('；')}。仍然使用吗？`,
      '密码强度提示',
      { type: 'warning', confirmButtonText: '仍然使用', cancelButtonText: '取消' },
    ).catch(() => 'cancel')
    if (ret === 'cancel') return
  } else if (level === 'medium') {
    ElMessage.info(`密码强度中等（还可：${tips.join('；')}）`)
  }
  saving.value = true
  try {
    await modifyEntry(
      entryStore.selectedDN,
      [{ op: 'replace', attr: 'userPassword', values: [pwd] }],
      { userPassword: pwdForm.value.algo },
    )
    ElMessage.success('密码已更新')
    showEditPwd.value = false
    loadEntry(entryStore.selectedDN)
  } finally { saving.value = false }
}

// ==================== 成员管理 helper（对标 phpLDAPadmin） ====================
/** 候选用户（成员选择器用）：DN、uid 和姓名 */
interface CandidateUser { dn: string; uid: string; cn: string }
/** 候选组（加入组对话框用）：DN、组名和 GID */
interface CandidateGroup { dn: string; cn: string; gidNumber: string }

const serverStore = useServerStore()  // 服务器 Store，用于获取搜索 Base DN
const serverList = ref<any[]>([])      // 所有服务器配置列表（跨服务器复制下拉用）
getServers().then(list => { serverList.value = list }).catch(() => {})

const showMemberPicker = ref(false)          // 成员选择器对话框
const memberFilter = ref('')                 // 成员过滤关键词
const memberSelection = ref<CandidateUser[]>([])   // 成员选择器中已勾选的用户
const candidateUsers = ref<CandidateUser[]>([])    // 成员选择器候选用户列表
const memberPickForm = ref({ attr: '', kind: 'dn' as 'dn' | 'uid', existing: [] as string[] })  // 成员选择上下文：目标属性、值类型（DN/uid）、已存在的值

const showJoinGroup = ref(false)             // 加入组对话框
const groupFilter = ref('')                  // 组过滤关键词
const groupSelection = ref<CandidateGroup[]>([])   // 加入组对话框中已勾选的组
const candidateGroups = ref<CandidateGroup[]>([])  // 加入组对话框候选组列表
const joinGroupKind = ref<'uid' | 'dn'>('uid') // 加入 posixGroup 用 uid，groupOfNames 用 DN

/** 当前条目是否为成员类属性（groupOfNames.member / posixGroup.memberUid 等） */
function isMemberAttr(n: string): boolean {
  return isMemberAttrShared(n)
}

/** 当前条目 objectClass 集合（小写） */
const ocSet = computed(() =>
  new Set((entry.value?.object_classes || []).map((o) => o.toLowerCase())),
)

/** 当前条目是否可以"加入组"：人/账户类条目 */
const canJoinGroup = computed(() =>
  ['person', 'organizationalperson', 'inetorgperson', 'posixaccount'].some((o) => ocSet.value.has(o)),
)

/** 是否为 posix 账户（需展示所属组列表） */
const isPosixUser = computed(() => ocSet.value.has('posixaccount'))

/** 当前用户的 uid */
const userUid = computed(() =>
  entry.value?.attributes?.find((a) => a.name.toLowerCase() === 'uid')?.values[0] || '',
)

interface UserGroup { dn: string; cn: string; gidNumber: string }
const userGroups = ref<UserGroup[]>([])

/** 名称解析映射：原始值 → 友好显示名（对标 phpLDAPadmin 把 uid/gid 解析为 cn） */
const resolvedNames = ref<Map<string, string>>(new Map())

/** 是否为 posix 组 */
const isPosixGroup = computed(() => ocSet.value.has('posixgroup'))

/** 解析成员 uid 和 gidNumber 为友好名称 — 对标 phpLDAPadmin 的名称解析 */
async function resolveNames() {
  const map = new Map<string, string>()
  const base = serverStore.currentServer()?.base_dn
  if (!base || !entry.value) { resolvedNames.value = map; return }
  const attrs = entry.value.attributes

  try {
    // posix 组：把所有 memberUid 解析为 cn（显示 "cn (uid)"）
    if (isPosixGroup.value) {
      const memberUids = attrs.find((a) => a.name.toLowerCase() === 'memberuid')?.values || []
      if (memberUids.length) {
        const filter = `(&(objectClass=posixAccount)(|${memberUids.map((u) => `(uid=${u})`).join('')}))`
        const users = await search({ base_dn: base, filter, scope: 'sub', size_limit: 500 })
        for (const u of users) {
          const uid = u.attributes.find((a) => a.name.toLowerCase() === 'uid')?.values[0]
          const cn = u.attributes.find((a) => a.name.toLowerCase() === 'cn')?.values[0]
          if (uid && cn) map.set(`memberuid:${uid}`, cn)
        }
      }
    }

    // posix 用户：把 gidNumber 解析为对应组的 cn（显示 "组名 (gid)"）
    if (isPosixUser.value) {
      const gid = attrs.find((a) => a.name.toLowerCase() === 'gidnumber')?.values[0]
      if (gid) {
        const groups = await search({
          base_dn: base,
          filter: `(&(objectClass=posixGroup)(gidNumber=${gid}))`,
          scope: 'sub',
          size_limit: 1,
        })
        if (groups.length) {
          const cn = groups[0].attributes.find((a) => a.name.toLowerCase() === 'cn')?.values[0]
          if (cn) map.set(`gidnumber:${gid}`, cn)
        }
      }
    }
  } catch { /* 解析失败则仅显示原始值 */ }

  resolvedNames.value = map
}

/** 获取属性值的解析名称（对标 phpLDAPadmin：uid→cn, gid→组名），未解析返回 null */
function resolvedName(attrName: string, val: string): string | null {
  const key = `${attrName.toLowerCase()}:${val}`
  return resolvedNames.value.get(key) || null
}

/** 加载 posix 用户所属的组（memberUid 包含该用户 uid 的 posixGroup）— 对标 phpLDAPadmin Groups 区块 */
async function loadUserGroups() {
  if (!isPosixUser.value || !userUid.value) { userGroups.value = []; return }
  const base = serverStore.currentServer()?.base_dn
  if (!base) { userGroups.value = []; return }
  try {
    const list = await search({
      base_dn: base,
      filter: `(&(objectClass=posixGroup)(memberUid=${userUid.value}))`,
      scope: 'sub',
      size_limit: 200,
    })
    userGroups.value = list.map((e) => ({
      dn: e.dn,
      cn: e.attributes.find((a) => a.name.toLowerCase() === 'cn')?.values[0] || e.dn,
      gidNumber: e.attributes.find((a) => a.name.toLowerCase() === 'gidnumber')?.values[0] || '',
    }))
  } catch {
    userGroups.value = []
  }
}

/** 成员列表过滤 */
const filteredCandidates = computed(() => {
  const k = memberFilter.value.trim().toLowerCase()
  if (!k) return candidateUsers.value
  return candidateUsers.value.filter(
    (u) => u.uid.toLowerCase().includes(k) || u.cn.toLowerCase().includes(k) || u.dn.toLowerCase().includes(k),
  )
})

/** 组列表过滤 */
const filteredGroups = computed(() => {
  const k = groupFilter.value.trim().toLowerCase()
  if (!k) return candidateGroups.value
  return candidateGroups.value.filter(
    (g) => g.cn.toLowerCase().includes(k) || g.dn.toLowerCase().includes(k),
  )
})

/** 加入组对话框的提示文案 */
const joinGroupHint = computed(() =>
  joinGroupKind.value === 'uid'
    ? '将把当前用户的 uid 添加到所选 posixGroup 的 memberUid 属性'
    : '将把当前用户的 DN 添加到所选组的 member 属性',
)

/** 从条目属性中取第一个指定属性值 */
function firstAttrValue(name: string): string {
  const attr = entry.value?.attributes?.find(
    (a) => a.name.toLowerCase() === name.toLowerCase(),
  )
  return attr?.values?.[0] || ''
}

/** 打开成员选择器，加载候选用户列表 */
async function openMemberPicker(attr: LdapAttribute) {
  const base = serverStore.currentServer()?.base_dn
  if (!base) { ElMessage.error('无法获取 Base DN'); return }

  const isUidKind = attr.name.toLowerCase() === 'memberuid'
  memberPickForm.value = { attr: attr.name, kind: isUidKind ? 'uid' : 'dn', existing: [...attr.values] }
  memberFilter.value = ''
  memberSelection.value = []
  showMemberPicker.value = true

  try {
    // posixGroup.memberUid 需要 posixAccount 的 uid；groupOfNames.member 需要 person 类条目的 DN
    const filter = isUidKind
      ? '(&(objectClass=posixAccount)(uid=*))'
      : '(&(objectClass=person)(|(objectClass=inetOrgPerson)(objectClass=organizationalPerson)(objectClass=posixAccount)))'
    const list = await search({ base_dn: base, filter, scope: 'sub', size_limit: 200 })
    candidateUsers.value = list.map((e) => ({
      dn: e.dn,
      uid: e.attributes.find((a) => a.name.toLowerCase() === 'uid')?.values[0] || '',
      cn: e.attributes.find((a) => a.name.toLowerCase() === 'cn')?.values[0] || e.dn,
    }))
  } catch (e) {
    ElMessage.error('加载候选用户失败')
    candidateUsers.value = []
  }
}

/** 确认添加成员：uid 或 DN 按属性类型追加，自动去重 */
async function confirmAddMembers() {
  const { attr, kind, existing } = memberPickForm.value
  let values = memberSelection.value.map((u) => (kind === 'uid' ? u.uid : u.dn)).filter(Boolean)
  values = values.filter((v) => !existing.includes(v))
  if (!values.length) {
    ElMessage.warning('所选成员已全部存在')
    return
  }
  saving.value = true
  try {
    await modifyEntry(entryStore.selectedDN, [{ op: 'add', attr, values }])
    ElMessage.success(`已添加 ${values.length} 个成员`)
    showMemberPicker.value = false
    loadEntry(entryStore.selectedDN)
  } catch {
    // 错误由请求层提示
  } finally { saving.value = false }
}

/** 移除单个成员值（对标 phpLDAPadmin 逐个移除组成员） */
async function removeMemberValue(attr: string, val: string) {
  if (!entryStore.selectedDN) return
  try {
    await modifyEntry(entryStore.selectedDN, [{ op: 'delete', attr, values: [val] }])
    ElMessage.success('已移除成员')
    loadEntry(entryStore.selectedDN)
  } catch { /* 错误由请求层提示 */ }
}

/** 打开加入组对话框，按用户类型加载组列表 */
async function openJoinGroup() {
  const base = serverStore.currentServer()?.base_dn
  if (!base) { ElMessage.error('无法获取 Base DN'); return }

  // posixAccount 加入 posixGroup 用 memberUid；其他用户加入 groupOfNames 用 member DN
  const isPosix = ocSet.value.has('posixaccount')
  joinGroupKind.value = isPosix ? 'uid' : 'dn'
  groupFilter.value = ''
  groupSelection.value = []
  showJoinGroup.value = true

  try {
    const filter = isPosix
      ? '(objectClass=posixGroup)'
      : '(|(objectClass=groupOfNames)(objectClass=groupOfUniqueNames))'
    const list = await search({
      base_dn: base,
      filter,
      scope: 'sub',
      attributes: ['cn', 'gidNumber'],
      size_limit: 200,
    })
    candidateGroups.value = list.map((e) => ({
      dn: e.dn,
      cn: e.attributes.find((a) => a.name.toLowerCase() === 'cn')?.values[0] || e.dn,
      gidNumber: e.attributes.find((a) => a.name.toLowerCase() === 'gidnumber')?.values[0] || '',
    }))
  } catch {
    ElMessage.error('加载组列表失败')
    candidateGroups.value = []
  }
}

/** 确认加入组：逐个组追加成员属性，已存在的组跳过并提示 */
async function confirmJoinGroups() {
  const dn = entryStore.selectedDN
  const isUid = joinGroupKind.value === 'uid'
  const value = isUid ? firstAttrValue('uid') : dn
  if (!value) {
    ElMessage.error(isUid ? '当前用户缺少 uid 属性' : '无法获取当前 DN')
    return
  }
  saving.value = true
  const attr = isUid ? 'memberUid' : 'member'
  let ok = 0
  const failed: string[] = []
  try {
    for (const g of groupSelection.value) {
      try {
        await modifyEntry(g.dn, [{ op: 'add', attr, values: [value] }])
        ok++
      } catch {
        failed.push(g.cn)
      }
    }
    if (ok) ElMessage.success(`已加入 ${ok} 个组`)
    if (failed.length) ElMessage.warning(`以下组加入失败（可能已是成员）: ${failed.join(', ')}`)
    showJoinGroup.value = false
  } finally { saving.value = false }
}

// ==================== 条目级操作 ====================
/** 删除当前条目（需用户确认，不可撤销） */
async function handleDelete() {
  if (!entryStore.selectedDN) return
  // 对标 phpLDAPadmin delete_form.php — 递归删除前先列出所有将被删除的子条目
  let descendantCount = 0
  try {
    const res = await listDescendants(entryStore.selectedDN)
    descendantCount = res.count
  } catch { /* 权限或无子孙 */ }

  const msg = descendantCount > 0
    ? `确定删除 "${entryStore.selectedDN}" 及其 ${descendantCount} 个子孙条目吗？此操作不可撤销。`
    : `确定删除条目 "${entryStore.selectedDN}" 吗？此操作不可撤销。`

  await ElMessageBox.confirm(msg, '删除条目', { type: 'error' })
  try {
    await apiDelete(entryStore.selectedDN, descendantCount > 0) // 有子孙才递归
    ElMessage.success('已删除')
    const parentDn = parentOf(entryStore.selectedDN)
    entryStore.selectEntry('')
    entryStore.refreshTree(parentDn) // 展开被删除条目的父节点
    router.push('/entry')
  } catch { /* ignore */ }
}
/** 打开复制对话框（对标 phpLDAPadmin copy_form.php — 带 EntryChooser + server 下拉） */
function handleCopy() {
  if (!entryStore.selectedDN) return
  copyForm.value = { destParent: parentOf(entryStore.selectedDN), newRDN: '', recursive: true, targetServerId: null }
  showCopy.value = true
}
/** 执行复制操作（对标 phpLDAPadmin copy_form.php — 支持跨服务器） */
async function doCopy() {
  if (!entryStore.selectedDN || !copyForm.value.destParent) {
    ElMessage.warning('请填写目标位置 DN')
    return
  }
  copying.value = true
  try {
    const srcRDN = entryStore.selectedDN.split(',').slice(0, 1).join(',')
    const destDN = copyForm.value.newRDN
      ? `${copyForm.value.newRDN},${copyForm.value.destParent}`
      : `${srcRDN},${copyForm.value.destParent}`
    await copyEntry(
      entryStore.selectedDN,
      destDN,
      copyForm.value.recursive,
      copyForm.value.targetServerId,
    )
    const serverHint = copyForm.value.targetServerId ? '（跨服务器）' : ''
    ElMessage.success(`已复制${serverHint}`)
    showCopy.value = false
    entryStore.refreshTree(copyForm.value.destParent)
  } finally { copying.value = false }
}
/** 执行条目移动操作 */
async function doMove() {
  if (!entryStore.selectedDN || !moveForm.value.newParent) return
  moving.value = true
  try {
    await moveEntry(entryStore.selectedDN, moveForm.value.newParent, moveForm.value.newRDN || undefined)
    ElMessage.success('已移动')
    showMove.value = false
    entryStore.refreshTree(moveForm.value.newParent) // 展开移动后的父节点
    entryStore.selectEntry('')
  } finally { moving.value = false }
}

/** 取 DN 的父 DN（去掉第一个 RDN 组件） */
function parentOf(dn: string): string {
  const idx = dn.indexOf(',')
  return idx > 0 ? dn.slice(idx + 1).trim() : dn
}
/** 导出当前条目为 LDIF 文件（exportLdif 内部已完成下载，无需重复触发） */
async function handleExport() {
  if (!entryStore.selectedDN) return
  try {
    const filename = await exportLdif(entryStore.selectedDN, true)
    ElMessage.success(`已下载 ${filename}`)
  } catch (e: any) {
    ElMessage.error(`导出失败: ${e?.message || e}`)
  }
}
/** 新建子条目对话框关闭后刷新 — 展开父节点使新条目可见 */
function onEntryChanged() {
  showCreate.value = false
  const parentDn = entryStore.selectedDN
  entryStore.refreshTree(parentDn) // 展开父节点，lazy 树会加载其下子条目
  loadEntry(parentDn)
}

// ==================== 路由查询参数处理 ====================
import { onMounted } from 'vue'
/** 挂载时读取路由查询参数中的 dn，选中对应条目（支持外部链接直达条目详情） */
onMounted(() => {
  const dn = route.query.dn as string
  if (dn) entryStore.selectEntry(dn)
})
</script>

<style scoped lang="scss">
.entry-view {
  height: 100%;
  display: flex; flex-direction: column;
  overflow: hidden;
}

/* Empty */
.empty {
  height: 100%;
  display: flex; align-items: center; justify-content: center;

  .empty-card {
    display: flex; flex-direction: column; align-items: center; gap: 14px;
    padding: 48px;
    color: var(--text-muted);
    text-align: center;

    svg { color: var(--neutral-300); }
    h3 { margin: 0; font-size: var(--fs-subhead); color: var(--text-secondary); }
    p  { margin: 0; font-size: var(--fs-meta); max-width: 320px; }
  }
}

/* Entry head */
.entry-head {
  display: flex; justify-content: space-between; align-items: flex-start;
  padding: 20px 24px;
  background: var(--surface-panel);
  border-bottom: 1px solid var(--border-subtle);
  gap: 20px;
  flex-shrink: 0;
}

.eh-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }

.dn-row {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-3);

  .dn-kicker {
    font-size: var(--fs-caption);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .dn-value {
    font-size: var(--fs-body);
    color: var(--text-primary);
    flex: 1; min-width: 0;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }

  .copy-btn {
    width: 26px; height: 26px;
    display: flex; align-items: center; justify-content: center;
    background: var(--surface-panel);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-1);
    color: var(--text-muted); cursor: pointer;
    transition: all 0.15s ease;
    flex-shrink: 0;

    &:hover { color: var(--accent-primary); border-color: var(--accent-primary); }
  }
}

.oc-row {
  display: flex; align-items: center; gap: 6px; flex-wrap: wrap;

  .oc-kicker {
    font-size: var(--fs-caption);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
    margin-right: 4px;
  }
}

.oc-chip {
  font-size: var(--fs-caption);
  font-family: var(--font-mono);
  padding: 2px 8px;
  background: color-mix(in oklab, var(--purple-500) 14%, white);
  color: var(--purple-500);
  border: 1px solid color-mix(in oklab, var(--purple-500) 22%, white);
  border-radius: var(--radius-1);
}

/* Toolbar */
.eh-tools {
  display: flex; align-items: center; gap: 6px; flex-shrink: 0;
}

.tb {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 8px 13px;
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  color: var(--text-secondary);
  font-size: var(--fs-meta);
  font-weight: 500;
  font-family: var(--font-ui);
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    background: var(--surface-hover);
    color: var(--text-primary);
    border-color: var(--border-default);
  }

  &.primary {
    background: var(--accent-primary);
    border-color: var(--accent-primary);
    color: var(--text-inverse);

    &:hover {
      background: var(--accent-primary-hover);
      border-color: var(--accent-primary-hover);
      box-shadow: var(--elevation-2);
    }
  }

  &.danger:hover {
    color: var(--accent-danger);
    border-color: color-mix(in oklab, var(--red-500) 40%, white);
    background: var(--accent-danger-soft);
  }
}

.tb-sep { width: 1px; height: 22px; background: var(--border-subtle); margin: 0 2px; }

/* Attributes area */
.attr-area {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}

/* posix 用户所属组区块 — 对标 phpLDAPadmin Groups */
.groups-area {
  padding: 16px 24px 0;

  .groups-list {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 12px;
  }

  .group-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    background: var(--surface-base);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-2);
    cursor: pointer;
    transition: border-color 0.15s ease, background 0.15s ease;

    &:hover {
      border-color: var(--primary, #409eff);
      background: var(--surface-hover, #f5f7fa);
    }

    .gc-cn { font-size: 13px; color: var(--text-primary); }
    .gc-gid { font-size: 11px; color: var(--text-muted); }
  }

  .groups-empty {
    margin-top: 10px;
    font-size: 12px;
    color: var(--text-muted);
  }
}

.attr-head {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 14px;
}

.ah-left {
  display: flex; align-items: center; gap: 10px;

  .ah-bar { width: 3px; height: 12px; border-radius: 2px; background: var(--accent-primary); }
  .ah-title {
    font-size: var(--fs-label);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
  }
  .ah-count {
    font-size: 11px;
    padding: 1px 8px;
    background: var(--surface-base);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-1);
    color: var(--text-muted);
  }
}

.ah-add {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 12px;
  background: transparent;
  border: 1px dashed var(--border-default);
  border-radius: var(--radius-2);
  color: var(--text-secondary);
  font-size: var(--fs-meta);
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    color: var(--accent-primary);
    border-color: var(--accent-primary);
    background: var(--accent-primary-soft);
  }
}

/* Attribute cards — grid */
.attr-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: 10px;
}

.attr-card {
  display: flex; align-items: center; gap: 12px;
  padding: 12px 14px;
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-3);
  transition: all 0.15s ease;

  &:hover {
    border-color: var(--border-default);
    box-shadow: var(--elevation-2);

    .aa-btn { opacity: 1; }
  }
}

.ac-left { display: flex; align-items: center; gap: 10px; min-width: 0; flex-shrink: 0; }

.ac-icon {
  width: 34px; height: 34px;
  border-radius: var(--radius-2);
  display: flex; align-items: center; justify-content: center;
  font-size: 14px;
  font-weight: 700;
  font-family: var(--font-mono);
  flex-shrink: 0;

  &.ic-primary {
    background: color-mix(in oklab, var(--accent-primary) 12%, white);
    color: var(--accent-primary);
  }
  &.ic-danger {
    background: color-mix(in oklab, var(--red-500) 14%, white);
    color: var(--red-600);
  }
  &.ic-success {
    background: color-mix(in oklab, var(--green-600) 14%, white);
    color: var(--green-700);
  }
  &.ic-purple {
    background: color-mix(in oklab, var(--purple-500) 14%, white);
    color: var(--purple-500);
  }
}

.ac-meta { display: flex; flex-direction: column; min-width: 0; gap: 2px; }

.ac-name {
  font-size: var(--fs-caption);
  font-weight: 400;
  color: var(--text-muted);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

/* 属性友好名称 — 对标 phpLDAPadmin 字段标签 */
.ac-friendly {
  font-size: var(--fs-body);
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

.ac-multi {
  font-size: var(--fs-caption);
  font-family: var(--font-mono);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted);
}

.ac-values {
  flex: 1; min-width: 0;
  display: flex; flex-direction: column; gap: 6px;
}

.tag-bin, .tag-pwd {
  display: inline-flex; align-items: center; gap: 5px;
  font-family: var(--font-mono);
}
.tag-bin {
  background: color-mix(in oklab, var(--purple-500) 14%, white);
  color: var(--purple-500);
  border: 1px solid color-mix(in oklab, var(--purple-500) 22%, white);
}
.tag-pwd {
  background: color-mix(in oklab, var(--red-500) 14%, white);
  color: var(--red-600);
  border: 1px solid color-mix(in oklab, var(--red-500) 22%, white);
}

.val-btn {
  font-size: var(--fs-meta);
  padding: 4px 10px;
  background: transparent;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-1);
  color: var(--text-secondary);
  cursor: pointer;
  align-self: flex-start;
  transition: all 0.15s ease;

  &:hover { color: var(--accent-primary); border-color: var(--accent-primary); }
}

.val-chips { display: flex; flex-wrap: wrap; gap: 5px; }

.val-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 9px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  max-width: 100%;

  code {
    font-size: 12px;
    font-family: var(--font-mono);
    color: var(--text-primary);
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
    display: inline-block; max-width: 340px;
  }

  &.val-chip-member {
    padding-right: 4px;

    .chip-remove {
      cursor: pointer;
      color: var(--text-muted);
      font-size: 12px;
      transition: color 0.15s ease;

      &:hover { color: var(--danger, #f56c6c); }
    }
  }

  /* 解析后的友好名称展示（uid→姓名, gid→组名） */
  &.val-chip-resolved {
    .vc-name { color: var(--text-primary); font-size: 12px; font-weight: 500; }
    .vc-raw { color: var(--text-muted); font-size: 11px; }
  }
}

.ac-actions { display: flex; gap: 4px; flex-shrink: 0; }

.aa-btn {
  width: 30px; height: 30px;
  display: flex; align-items: center; justify-content: center;
  background: transparent; border: 1px solid transparent;
  border-radius: var(--radius-1);
  color: var(--text-muted);
  cursor: pointer;
  opacity: 0.4;
  transition: all 0.15s ease;

  &:hover {
    opacity: 1;
    color: var(--text-primary);
    background: var(--surface-hover);
    border-color: var(--border-subtle);
  }

  &.danger:hover {
    color: var(--accent-danger);
    background: var(--accent-danger-soft);
    border-color: color-mix(in oklab, var(--red-500) 22%, white);
  }
}

/* Empty */
.attr-empty {
  grid-column: 1 / -1;
  text-align: center;
  padding: 40px;
  color: var(--text-muted);

  p { margin: 0 0 10px; font-size: var(--fs-meta); }
}

.add-attr-sm {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 12px;
  background: transparent;
  border: 1px dashed var(--border-default);
  border-radius: var(--radius-2);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover { color: var(--accent-primary); border-color: var(--accent-primary); }
}

/* Dialog internals */
.dlg-body {
  display: flex; flex-direction: column; gap: 16px;
}
.dlg-row {
  display: flex; flex-direction: column; gap: 6px;
}
.dlg-label {
  font-size: var(--fs-label);
  font-weight: 500;
  color: var(--text-secondary);
}
.dlg-val {
  padding: 8px 12px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  font-size: var(--fs-body);
  color: var(--text-primary);
}

.dlg-input-row {
  display: flex; gap: 8px;
  :deep(.el-input) { flex: 1; }
}

.pick-btn {
  width: 36px; height: 32px;
  display: inline-flex; align-items: center; justify-content: center;
  background: var(--surface-base); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all .15s;
  flex-shrink: 0;
  &:hover { color: var(--accent-primary); border-color: var(--accent-primary); background: var(--accent-primary-soft); }
}

.dlg-hint {
  font-size: 11px; color: var(--amber-600, #b45309); margin-top: 4px;
}

/* LDAP Compare 结果 — 对标 phpLDAPadmin compare.php */
.compare-result {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 14px; border-radius: 6px; font-size: 13px; font-weight: 500;
  &.match { background: color-mix(in oklab, var(--green-500) 14%, white); color: var(--green-700, #15803d); }
  &.nomatch { background: color-mix(in oklab, var(--red-500) 14%, white); color: var(--red-700, #b91c1c); }
}

/* 二进制属性上传区 — 对标 phpLDAPadmin 上传表单 */
.upload-area {
  display: flex; flex-direction: column; gap: 8px;
}
.upload-preview {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 12px;
  background: var(--surface-base); border: 1px solid var(--border-subtle);
  border-radius: 6px;
  font-size: 12px; color: var(--text-secondary);
}

.val-edit-list { display: flex; flex-direction: column; gap: 8px; }
.val-edit-row { display: flex; gap: 8px; align-items: flex-start; }
.val-edit-del {
  width: 34px; height: 34px;
  display: flex; align-items: center; justify-content: center;
  background: var(--accent-danger-soft);
  border: 1px solid color-mix(in oklab, var(--red-500) 22%, white);
  border-radius: var(--radius-2);
  color: var(--accent-danger);
  cursor: pointer;
  flex-shrink: 0;
  transition: all 0.15s ease;

  &:hover { background: color-mix(in oklab, var(--red-500) 20%, white); }
}

.add-value-btn {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 12px;
  background: transparent;
  border: 1px dashed var(--border-default);
  border-radius: var(--radius-2);
  color: var(--text-muted);
  font-size: var(--fs-meta);
  cursor: pointer;
  align-self: flex-start;
  transition: all 0.15s ease;

  &:hover { color: var(--accent-primary); border-color: var(--accent-primary); }
}

/* 成员/组选择器提示 */
.picker-hint {
  font-size: var(--fs-caption);
  color: var(--text-muted);
  line-height: 1.5;
}

/* === 二进制属性预览区 — 对标 phpLDAPadmin view_jpeg_photo.php / download_binary_attr.php === */
.bin-preview {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);

  img {
    width: 48px; height: 48px;
    object-fit: cover;
    border-radius: var(--radius-1);
    border: 1px solid var(--border-subtle);
  }

  .bin-size {
    font-size: 11px;
    color: var(--text-muted);
  }
}

.bin-generic {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  color: var(--purple-500);

  .bin-size {
    font-size: 11px;
    color: var(--text-muted);
  }
}

.bin-loading {
  display: inline-flex; align-items: center; gap: 6px;
  font-size: var(--fs-caption); color: var(--text-muted);
  .spin { animation: spin 1s linear infinite; }
}

.bin-missing {
  font-size: var(--fs-caption);
  color: var(--text-muted);
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
