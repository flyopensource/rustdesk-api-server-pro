<script setup lang="tsx">
import { computed, onMounted, reactive, ref } from 'vue';
import { NButton, NFlex, NSelect, NTag } from 'naive-ui';
import {
  createDesktopEnrollmentToken,
  createDeviceGroup,
  createServerProfile,
  deleteDeviceGroup,
  deleteDeviceRecord,
  deleteServerProfile,
  fetchDesktopEnrollmentTokens,
  fetchDeviceAlias,
  fetchDeviceGroups,
  fetchDevicesList,
  fetchServerProfilePreview,
  fetchServerProfiles,
  revokeDesktopEnrollmentToken,
  updateDeviceAlias,
  updateDeviceEnabled,
  updateDeviceGroup,
  updateDeviceGroupAssignment,
  updateServerProfile
} from '@/service/api/devices';
import { $t } from '@/locales';
import { useAppStore } from '@/store/modules/app';
import { useTable } from '@/hooks/common/table';
import TableHeader from './components/table-header.vue';
import AuditBaseLogsSearch from './components/search.vue';

const appStore = useAppStore();
const profiles = ref<Api.Devices.ServerProfile[]>([]);
const groups = ref<Api.Devices.DeviceGroup[]>([]);
const unassignedDeviceCount = ref(0);
const profilesVisible = ref(false);
const groupsVisible = ref(false);
const previewVisible = ref(false);
const preview = ref<Api.Devices.ServerProfilePreview | null>(null);
const enrollmentTokensVisible = ref(false);
const enrollmentTokensLoading = ref(false);
const enrollmentTokenSaving = ref(false);
const enrollmentTokens = ref<Api.Devices.DesktopEnrollmentToken[]>([]);
const createdEnrollmentToken = ref<Api.Devices.CreatedDesktopEnrollmentToken | null>(null);
const enrollmentTokenForm = reactive<{ group_id: number; note: string; expires_at: number | null }>({
  group_id: 0,
  note: '',
  expires_at: Date.now() + 7 * 24 * 60 * 60 * 1000
});
const currentApiServer = window.location.origin;
const aliasVisible = ref(false);
const deleteVisible = ref(false);
const deleteSaving = ref(false);
let refreshDeviceList: () => void | Promise<void> = () => {};
const deleteForm = reactive({ id: 0, rustdesk_id: '', confirmation: '' });

function confirmDelete(row: Api.Devices.Device) {
  Object.assign(deleteForm, { id: row.id!, rustdesk_id: row.rustdesk_id, confirmation: '' });
  deleteVisible.value = true;
}

async function removeDevice() {
  if (deleteForm.confirmation !== deleteForm.rustdesk_id) return;
  deleteSaving.value = true;
  try {
    const { error } = await deleteDeviceRecord(deleteForm.id, deleteForm.confirmation);
    if (error) return;
    deleteVisible.value = false;
    window.$message?.success('设备管理记录已删除');
    await refreshDeviceList();
  } finally {
    deleteSaving.value = false;
  }
}
const aliasSaving = ref(false);
const aliasForm = reactive({ id: 0, rustdesk_id: '', alias: '', address_book_ids: [] as number[] });
const aliasTargets = ref<Api.Devices.DeviceAlias['targets']>([]);
const aliasOptions = computed(() => aliasTargets.value.map(target => ({
  value: target.id, label: `${target.username} · ${target.name}${target.enabled ? '' : '（账号停用）'}`, disabled: !target.enabled
})));

async function editAlias(row: Api.Devices.Device) {
  const { data: result, error } = await fetchDeviceAlias(row.id!);
  if (error || !result) return;
  Object.assign(aliasForm, { id: row.id!, rustdesk_id: row.rustdesk_id, alias: result.alias, address_book_ids: [...result.address_book_ids] });
  aliasTargets.value = result.targets;
  aliasVisible.value = true;
}

async function saveAlias() {
  aliasSaving.value = true;
  try {
    const { error } = await updateDeviceAlias({ id: aliasForm.id, alias: aliasForm.alias, address_book_ids: aliasForm.address_book_ids });
    if (error) return;
    aliasVisible.value = false;
    window.$message?.success('已保存到所选地址簿，请在官方客户端刷新地址簿');
    await refreshDeviceList();
  } finally {
    aliasSaving.value = false;
  }
}

const profileForm = reactive({
  id: 0,
  name: '',
  id_server: '',
  relay_server: '',
  server_key: '',
  enabled: true
});
const groupForm = reactive({
  id: 0,
  name: '',
  enabled: true,
  is_default: false,
  profile_id: 0,
  unattended_enabled: false,
  permanent_password: '',
  clear_password: false,
  password_set: false,
  root_command: 'auto'
});

function toggleDevice(row: Api.Devices.Device) {
  window.$dialog?.warning({
    title: row.disabled ? '重新启用设备' : '停用设备',
    content: `${row.rustdesk_id}：${row.disabled ? '恢复管理 API 访问' : '停止管理 API 访问及后续策略获取，不会断开已有远程会话'}`,
    positiveText: '确认', negativeText: '取消',
    onPositiveClick: async () => {
      const { error } = await updateDeviceEnabled(row.id!, row.disabled);
      if (error) return false;
      await refreshDeviceList();
      return true;
    }
  });
}

const serverProfileOptions = computed(() => [
  { label: '请选择服务器配置', value: 0, disabled: true },
  ...profiles.value.map(item => ({
    label: item.enabled ? item.name : `${item.name}（已停用）`,
    value: item.id,
    disabled: !item.enabled
  }))
]);

const groupOptions = computed(() => [
  { label: '未分组', value: 0 },
  ...groups.value.map(item => ({
    label: item.enabled && item.profile_enabled ? item.name : `${item.name}（不可用）`,
    value: item.id,
    disabled: !item.enabled || !item.profile_enabled
  }))
]);
const editingGroupImpact = computed(() => groups.value.find(item => item.id === groupForm.id));
const enrollmentGroupOptions = computed(() => [
  { label: '注册后不指定设备组', value: 0 },
  ...groups.value.map(item => ({
    label: item.enabled ? item.name : `${item.name}（已停用）`,
    value: item.id,
    disabled: !item.enabled
  }))
]);

function formatDate(value: string) {
  if (!value || value.startsWith('0001-')) return '-';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString('zh-CN', { hour12: false });
}

function enrollmentStatusLabel(status: Api.Devices.DesktopEnrollmentToken['status']) {
  return { unused: '未使用', consumed: '已使用', revoked: '已撤销', expired: '已过期' }[status];
}

function enrollmentStatusType(status: Api.Devices.DesktopEnrollmentToken['status']): 'success' | 'warning' | 'error' | 'default' {
  return { unused: 'success', consumed: 'default', revoked: 'error', expired: 'warning' }[status] as
    | 'success'
    | 'warning'
    | 'error'
    | 'default';
}

function passwordStatusLabel(status: string) {
  return {
    unchanged: '无需变更',
    applying: '应用中',
    success: '下发成功',
    cleared: '已清除',
    failed: '失败'
  }[status] || '未上报';
}

function passwordStatusType(status: string): 'success' | 'warning' | 'error' | 'info' | 'default' {
  if (status === 'success' || status === 'cleared' || status === 'unchanged') return 'success';
  if (status === 'failed') return 'error';
  if (status === 'applying') return 'info';
  return 'default';
}

function profileStatusLabel(status: string) {
  return {
    idle: '等待应用',
    applying: '应用中',
    success: '应用成功',
    failed: '失败',
    rolled_back: '已回滚',
    disabled: '已停用'
  }[status] || '未上报';
}

function profileStatusType(status: string): 'success' | 'warning' | 'error' | 'info' | 'default' {
  if (status === 'success' || status === 'disabled') return 'success';
  if (status === 'failed' || status === 'rolled_back') return 'error';
  if (status === 'applying') return 'info';
  return 'default';
}

async function loadEnrollmentTokens() {
  enrollmentTokensLoading.value = true;
  try {
    const { data: result } = await fetchDesktopEnrollmentTokens();
    enrollmentTokens.value = result?.tokens || [];
  } finally {
    enrollmentTokensLoading.value = false;
  }
}

async function openEnrollmentTokens() {
  createdEnrollmentToken.value = null;
  enrollmentTokensVisible.value = true;
  await loadEnrollmentTokens();
}

async function createEnrollmentToken() {
  if (!enrollmentTokenForm.expires_at || enrollmentTokenForm.expires_at <= Date.now()) {
    window.$message?.warning('请选择未来的失效时间');
    return;
  }
  enrollmentTokenSaving.value = true;
  try {
    const { data: result, error } = await createDesktopEnrollmentToken({
      group_id: enrollmentTokenForm.group_id,
      note: enrollmentTokenForm.note.trim(),
      expires_at: Math.floor(enrollmentTokenForm.expires_at / 1000)
    });
    if (error || !result) return;
    createdEnrollmentToken.value = result;
    enrollmentTokenForm.note = '';
    window.$message?.success('一次性桌面注册令牌已创建');
    await loadEnrollmentTokens();
  } finally {
    enrollmentTokenSaving.value = false;
  }
}

async function copyEnrollmentToken() {
  if (!createdEnrollmentToken.value) return;
  try {
    await navigator.clipboard.writeText(createdEnrollmentToken.value.token);
    window.$message?.success('令牌已复制');
  } catch {
    window.$message?.error('复制失败，请手动复制');
  }
}

function revokeEnrollmentToken(token: Api.Devices.DesktopEnrollmentToken) {
  window.$dialog?.warning({
    title: '撤销桌面注册令牌',
    content: `确认撤销 ${token.selector}？撤销后不能恢复。`,
    positiveText: '确认撤销',
    negativeText: '取消',
    onPositiveClick: async () => {
      const { error } = await revokeDesktopEnrollmentToken(token.id);
      if (error) return false;
      window.$message?.success('令牌已撤销');
      await loadEnrollmentTokens();
      return true;
    }
  });
}

async function loadStrategy() {
  const [profileResponse, groupResponse] = await Promise.all([fetchServerProfiles(), fetchDeviceGroups()]);
  profiles.value = profileResponse.data?.profiles || [];
  groups.value = groupResponse.data?.groups || [];
  unassignedDeviceCount.value = groupResponse.data?.unassigned_device_count || 0;
}

function newProfile() {
  Object.assign(profileForm, {
    id: 0,
    name: '',
    id_server: '',
    relay_server: '',
    server_key: '',
    enabled: true
  });
}

function editProfile(profile: Api.Devices.ServerProfile) {
  Object.assign(profileForm, profile);
}

async function saveProfile() {
  const payload: Api.Devices.ServerProfileInput = {
    name: profileForm.name.trim(),
    id_server: profileForm.id_server.trim(),
    relay_server: profileForm.relay_server.trim(),
    server_key: profileForm.server_key.trim(),
    enabled: profileForm.enabled
  };
  const response = profileForm.id
    ? await updateServerProfile({ id: profileForm.id, ...payload })
    : await createServerProfile(payload);
  if (!response.error) {
    window.$message?.success('服务器配置已保存');
    newProfile();
    await loadStrategy();
    await refreshDeviceList();
  }
}

async function removeProfile(id: number) {
  const { error } = await deleteServerProfile(id);
  if (!error) {
    window.$message?.success('服务器配置已删除');
    if (profileForm.id === id) newProfile();
    await loadStrategy();
    await refreshDeviceList();
  }
}

async function assignDeviceGroup(row: Api.Devices.Device, groupId: number) {
  const { error } = await updateDeviceGroupAssignment(row.id!, groupId);
  if (!error) {
    window.$message?.success(groupId ? '设备组已切换' : '设备已移出分组');
    await loadStrategy();
    await refreshDeviceList();
  }
}

function newGroup() {
  Object.assign(groupForm, {
    id: 0,
    name: '',
    enabled: true,
    is_default: false,
    profile_id: 0,
    unattended_enabled: false,
    permanent_password: '',
    clear_password: false,
    password_set: false,
    root_command: 'auto'
  });
}

function editGroup(group: Api.Devices.DeviceGroup) {
  Object.assign(groupForm, {
    id: group.id,
    name: group.name,
    enabled: group.enabled,
    is_default: group.is_default,
    profile_id: group.profile_id,
    unattended_enabled: group.unattended_enabled,
    permanent_password: '',
    clear_password: false,
    password_set: group.password_set,
    root_command: group.root_command || 'auto'
  });
}

function updateGroupEnabled(enabled: boolean) {
  groupForm.enabled = enabled;
  if (!enabled) groupForm.is_default = false;
}

async function saveGroup() {
  const base: Api.Devices.DeviceGroupInput = {
    name: groupForm.name.trim(),
    enabled: groupForm.enabled,
    is_default: groupForm.is_default,
    profile_id: groupForm.profile_id,
    unattended_enabled: groupForm.unattended_enabled,
    clear_password: groupForm.clear_password,
    root_command: groupForm.root_command.trim() || 'auto'
  };
  if (!groupForm.clear_password && groupForm.permanent_password) {
    base.permanent_password = groupForm.permanent_password;
  }
  const response = groupForm.id
    ? await updateDeviceGroup({ id: groupForm.id, ...base })
    : await createDeviceGroup(base);
  if (response.error) return;
  window.$message?.success('设备组已保存');
  newGroup();
  await loadStrategy();
  await refreshDeviceList();
}

async function removeGroup(id: number) {
  if (!(await deleteDeviceGroup(id)).error) {
    window.$message?.success('设备组已删除');
    await loadStrategy();
    await refreshDeviceList();
  }
}

async function showPreview(row: Api.Devices.Device) {
  const { data: result } = await fetchServerProfilePreview(row.id!);
  if (result) {
    preview.value = result;
    previewVisible.value = true;
  }
}

const {
  columns,
  columnChecks,
  data,
  getData,
  getDataByPage,
  loading,
  mobilePagination,
  searchParams,
  resetSearchParams
} = useTable({
  apiFn: fetchDevicesList,
  showTotal: true,
  apiParams: { current: 1, size: 10, hostname: null, username: null, rustdesk_id: null, state: null, alias: null },
  columns: () => [
    { key: 'id', width: 70, title: 'ID', align: 'center' },
    { key: 'rustdesk_id', width: 190, title: $t('dataMap.device.rustdesk_id'), align: 'center', ellipsis: { tooltip: true } },
    { key: 'alias', width: 160, title: '设备别名', align: 'center', render: row => (
      <NFlex vertical size={4} align="center">
        <span class="max-w-full break-all">{row.alias || '未命名'}</span>
        <NButton size="small" onClick={() => editAlias(row)}>编辑别名</NButton>
      </NFlex>
    ) },
    { key: 'hostname', width: 180, title: $t('dataMap.device.hostname'), align: 'center', ellipsis: { tooltip: true } },
    { key: 'username', width: 140, title: $t('dataMap.device.username'), align: 'center', ellipsis: { tooltip: true } },
    { key: 'version', width: 130, title: $t('dataMap.device.version'), align: 'center', ellipsis: { tooltip: true } },
    { key: 'platform', width: 170, title: '客户端类型', align: 'center', render: row => (
      <NFlex vertical size={4} align="center">
        <span>{row.platform || row.os || '-'}/{row.arch || '-'}</span>
        <NTag size="small" type={row.registration_type === 'desktop_token' ? 'success' : 'default'}>
          {row.registration_type === 'desktop_token' ? '桌面令牌注册' : row.registration_type || '旧版注册'}
        </NTag>
      </NFlex>
    ) },
    { key: 'disabled', width: 130, title: '管理状态', align: 'center', render: row => (
      <NFlex vertical size={4} align="center">
        <NTag type={row.disabled ? 'warning' : 'success'}>{row.disabled ? '已停用' : '正常'}</NTag>
        <span>{row.is_online ? '在线' : '离线'}</span>
        <NButton size="small" onClick={() => toggleDevice(row)}>{row.disabled ? '重新启用' : '停用'}</NButton>
        {row.disabled ? <NButton size="small" type="error" disabled={row.is_online} onClick={() => confirmDelete(row)}>删除记录</NButton> : null}
      </NFlex>
    ) },
    {
      key: 'group_name',
      width: 190,
      title: '设备组',
      align: 'center',
      render: row => (
        <NFlex vertical size={4} align="center">
          <NSelect
            class="w-full"
            value={row.group_id || 0}
            options={groupOptions.value}
            onUpdateValue={value => assignDeviceGroup(row, value)}
          />
          {row.group_id && !row.group_enabled ? (
            <NTag size="small" type="warning">
              所属组已停用
            </NTag>
          ) : null}
          <span class="max-w-full break-all text-12px">
            生效：{row.effective_group_name || '无管理策略'}
          </span>
          {row.group_warning ? (
            <NTag size="small" type="warning">
              {row.group_source === 'default' ? '已回退默认组' : '无可用策略'}
            </NTag>
          ) : null}
        </NFlex>
      )
    },
    {
      key: 'profile_name',
      width: 220,
      title: '服务器配置',
      align: 'center',
      render: row => (
        <NFlex vertical size={4} align="center">
          <span class="max-w-full break-all text-12px">
            {row.profile_name || '无配置'} · {row.profile_id_server || '-'}
          </span>
          <NButton size="tiny" onClick={() => showPreview(row)}>
            查看生效值
          </NButton>
        </NFlex>
      )
    },
    {
      key: 'unattended_enabled',
      width: 120,
      title: '无人值守',
      align: 'center',
      render: row => (
        <NFlex vertical size={4} align="center">
          <NTag size="small" type={row.unattended_enabled ? 'success' : 'default'}>
            {row.unattended_enabled ? '已开启' : '未开启'}
          </NTag>
          <span class="text-12px">
            {row.root_command === 'auto' ? 'auto（su/testsu）' : row.root_command || 'auto'}
          </span>
          <span class="text-12px">固定密码：{row.profile_password_set ? '已设置' : '未设置'}</span>
        </NFlex>
      )
    },
    {
      key: 'password_apply_status',
      width: 220,
      title: '桌面固定密码',
      align: 'center',
      render: row => row.registration_type === 'desktop_token' ? (
        <NFlex vertical size={4} align="center">
          <NTag size="small" type={passwordStatusType(row.password_apply_status)}>
            {passwordStatusLabel(row.password_apply_status)}
          </NTag>
          <span class="text-12px">设备密码：{row.permanent_password_set ? '已设置' : '未设置'}</span>
          <span class="text-12px">版本：{row.password_applied_revision || 0}/{row.policy_revision || 0}</span>
          {row.password_error ? <span class="max-w-full break-all text-12px text-error">{row.password_error}</span> : null}
          <span class="text-12px">{formatDate(row.password_reported_at)}</span>
        </NFlex>
      ) : <span class="text-12px">不适用</span>
    },
    {
      key: 'profile_connected',
      width: 230,
      title: '应用状态',
      align: 'center',
      render: row => row.registration_type === 'desktop_token' ? (
        <NFlex vertical size={4} align="center">
          <NTag size="small" type={profileStatusType(row.profile_apply_status)}>
            {profileStatusLabel(row.profile_apply_status)} / {row.profile_active_source || '未上报'}
          </NTag>
          <NTag size="small" type={row.profile_connected ? 'success' : 'default'}>
            {row.profile_connected ? '已连接' : '未连接'}
          </NTag>
          <span class="text-12px">
            版本：{row.profile_received_revision || 0}/{row.profile_applied_revision || 0}/{row.profile_failed_revision || 0}
          </span>
          {row.profile_fingerprint ? <span class="text-12px">指纹：{row.profile_fingerprint}</span> : null}
          {row.profile_error ? <span class="max-w-full break-all text-12px text-error">{row.profile_error}</span> : null}
          <span class="text-12px">{formatDate(row.profile_reported_at)}</span>
        </NFlex>
      ) : (
        <NFlex vertical size={4} align="center">
          <NTag size="small" type={row.profile_connected ? 'success' : 'default'}>
            {row.profile_active_source || '未上报'} / {row.profile_connected ? '已连接' : '未连接'}
          </NTag>
          <span class="text-12px">
            {row.profile_applied_revision || 0}/{row.policy_revision || 0}
          </span>
          {row.unattended_error ? <span class="max-w-full break-all text-12px text-error">{row.unattended_error}</span> : null}
          <NTag size="small" type={row.all_files_access_ready ? 'success' : 'warning'}>
            文件权限：{row.all_files_access_ready ? '就绪' : '未就绪'}
          </NTag>
          <span class="text-12px">{row.unattended_reported_at}</span>
        </NFlex>
      )
    }
  ]
});
const tableScrollX = computed(() => columns.value.reduce((total, column) => total + Number(column.width || 0), 0));
refreshDeviceList = getData;

onMounted(loadStrategy);
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <AuditBaseLogsSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />

    <NCard :title="$t('route.devices')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
      <template #header-extra>
        <NFlex align="center">
          <NButton size="small" type="primary" @click="openEnrollmentTokens">桌面注册令牌</NButton>
          <NButton size="small" @click="profilesVisible = true">服务器配置</NButton>
          <NButton size="small" @click="groupsVisible = true">设备组</NButton>
          <TableHeader v-model:columns="columnChecks" :loading="loading" @refresh="getData" />
        </NFlex>
      </template>
      <NDataTable
        :columns="columns"
        :data="data"
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="tableScrollX"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="device-table sm:h-full"
      />
    </NCard>

    <NModal v-model:show="profilesVisible" preset="card" title="服务器配置" class="max-w-95vw w-900px">
      <NAlert type="info" class="mb-16px">API Server 固定为当前注册地址：{{ currentApiServer }}，不随配置切换。</NAlert>
      <NGrid :cols="2" :x-gap="16" responsive="screen" item-responsive>
        <NGi span="2 m:1">
          <NForm label-placement="left" label-width="100">
            <NFormItem label="配置名称"><NInput v-model:value="profileForm.name" /></NFormItem>
            <NFormItem label="ID Server"><NInput v-model:value="profileForm.id_server" /></NFormItem>
            <NFormItem label="Relay Server"><NInput v-model:value="profileForm.relay_server" /></NFormItem>
            <NFormItem label="Server Key"><NInput v-model:value="profileForm.server_key" /></NFormItem>
            <NFormItem label="启用"><NSwitch v-model:value="profileForm.enabled" /></NFormItem>
            <NFlex justify="end">
              <NButton @click="newProfile">新建</NButton>
              <NButton type="primary" @click="saveProfile">保存配置</NButton>
            </NFlex>
          </NForm>
        </NGi>
        <NGi span="2 m:1">
          <NList bordered>
            <NListItem v-for="item in profiles" :key="item.id">
              <NThing
                :title="item.name"
                :description="`${item.id_server} / ${item.relay_server || '-'} / ${item.enabled ? '启用' : '停用'} · ${item.group_count} 个设备组`"
              />
              <template #suffix>
                <NFlex>
                  <NButton size="small" @click="editProfile(item)">编辑</NButton>
                  <NButton size="small" type="error" @click="removeProfile(item.id)">删除</NButton>
                </NFlex>
              </template>
            </NListItem>
          </NList>
        </NGi>
      </NGrid>
    </NModal>

    <NModal v-model:show="groupsVisible" preset="card" title="设备组管理" class="max-w-95vw w-820px">
      <NForm label-placement="left" label-width="100">
        <NAlert v-if="editingGroupImpact || groupForm.is_default" type="warning" class="mb-16px" :show-icon="false">
          保存后<span v-if="editingGroupImpact">会影响直接分组的 {{ editingGroupImpact.member_count }} 台设备</span><span v-if="editingGroupImpact && groupForm.is_default">，并默认覆盖 {{ unassignedDeviceCount }} 台未分组设备</span><span v-else-if="groupForm.is_default">会默认覆盖 {{ unassignedDeviceCount }} 台未分组设备</span>，设备下次心跳后获取新策略。
        </NAlert>
        <NFormItem label="组名"><NInput v-model:value="groupForm.name" /></NFormItem>
        <NFormItem label="服务器配置">
          <NSelect v-model:value="groupForm.profile_id" :options="serverProfileOptions" />
        </NFormItem>
        <NFormItem label="无人值守"><NSwitch v-model:value="groupForm.unattended_enabled" /></NFormItem>
        <NFormItem label="固定密码">
          <NInput
            v-model:value="groupForm.permanent_password"
            type="password"
            :disabled="groupForm.clear_password"
            :placeholder="groupForm.password_set ? '已设置，留空保持不变' : '未设置'"
          />
        </NFormItem>
        <NFormItem v-if="groupForm.password_set" label="清除密码">
          <NSwitch v-model:value="groupForm.clear_password" />
        </NFormItem>
        <NFormItem label="Root 执行器">
          <NInput v-model:value="groupForm.root_command" placeholder="auto、su、testsu 或绝对路径" />
        </NFormItem>
        <NAlert type="info" class="mb-16px" :show-icon="false">
          auto 会依次探测 su 和 testsu；明确知道主板执行器时可直接填写。关闭无人值守后，固定密码保留在设备组但不会下发。
        </NAlert>
        <NFormItem label="设备组状态"><NSwitch :value="groupForm.enabled" @update:value="updateGroupEnabled" /></NFormItem>
        <NFormItem label="默认设备组">
          <NSwitch v-model:value="groupForm.is_default" :disabled="!groupForm.enabled" />
          <span class="ml-12px text-12px">开启后，只有未分组设备默认使用该组策略。</span>
        </NFormItem>
        <NFlex justify="end">
          <NButton @click="newGroup">新建</NButton>
          <NButton type="primary" @click="saveGroup">保存设备组</NButton>
        </NFlex>
      </NForm>
      <NDivider />
      <NList bordered>
        <NListItem v-for="group in groups" :key="group.id">
          {{ group.name }}{{ group.is_default ? '（默认）' : '' }} · 设备组{{ group.enabled ? '启用' : '停用' }} ·
          直接分组 {{ group.member_count }} 台<span v-if="group.is_default"> · 默认覆盖 {{ group.default_coverage_count }} 台</span> ·
          {{ group.profile_name || '服务器配置不存在' }}{{ group.profile_enabled ? '' : '（不可用）' }} ·
          无人值守{{ group.unattended_enabled ? '开启' : '关闭' }} · 固定密码{{ group.password_set ? '已设置' : '未设置' }}
          <template #suffix>
            <NFlex>
              <NTag v-if="!group.configuration_complete" size="small" type="warning">策略不完整</NTag>
              <NButton size="small" @click="editGroup(group)">编辑</NButton>
              <NButton size="small" type="error" @click="removeGroup(group.id)">删除</NButton>
            </NFlex>
          </template>
        </NListItem>
      </NList>
    </NModal>

    <NModal v-model:show="enrollmentTokensVisible" preset="card" title="一次性桌面注册令牌" class="max-w-95vw w-980px">
      <NAlert type="warning" class="mb-16px" :show-icon="false">
        明文令牌只在创建成功后显示这一次。每个令牌只能注册一台桌面设备；请不要截图、写入日志或提交到代码仓库。
      </NAlert>
      <NForm label-placement="left" label-width="100">
        <NGrid :cols="2" :x-gap="16" responsive="screen" item-responsive>
          <NFormItemGi span="2 m:1" label="注册后设备组">
            <NSelect v-model:value="enrollmentTokenForm.group_id" :options="enrollmentGroupOptions" />
          </NFormItemGi>
          <NFormItemGi span="2 m:1" label="失效时间">
            <NDatePicker v-model:value="enrollmentTokenForm.expires_at" type="datetime" :clearable="false" class="w-full" />
          </NFormItemGi>
        </NGrid>
        <NFormItem label="备注">
          <NInput v-model:value="enrollmentTokenForm.note" maxlength="255" show-count placeholder="用途、交付对象或工单号；不要填写密码" />
        </NFormItem>
        <NFlex justify="end">
          <NButton type="primary" :loading="enrollmentTokenSaving" @click="createEnrollmentToken">创建一次性令牌</NButton>
        </NFlex>
      </NForm>

      <NAlert v-if="createdEnrollmentToken" type="success" class="my-16px" title="请立即复制并妥善交付">
        <NFlex vertical>
          <NInput :value="createdEnrollmentToken.token" readonly />
          <NFlex justify="end"><NButton type="primary" @click="copyEnrollmentToken">复制令牌</NButton></NFlex>
        </NFlex>
      </NAlert>

      <NDivider>最近令牌（不含明文）</NDivider>
      <NSpin :show="enrollmentTokensLoading">
        <NTable striped :single-line="false" size="small">
          <thead><tr><th>选择器</th><th>设备组</th><th>备注</th><th>状态</th><th>失效时间</th><th>使用设备</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="token in enrollmentTokens" :key="token.id">
              <td>{{ token.selector }}</td>
              <td>{{ groups.find(group => group.id === token.group_id)?.name || (token.group_id ? `#${token.group_id}` : '未指定') }}</td>
              <td class="max-w-240px break-all">{{ token.note || '-' }}</td>
              <td><NTag size="small" :type="enrollmentStatusType(token.status)">{{ enrollmentStatusLabel(token.status) }}</NTag></td>
              <td>{{ formatDate(token.expires_at) }}</td>
              <td>{{ token.consumed_device_id || '-' }}</td>
              <td><NButton v-if="token.status === 'unused'" size="small" type="error" @click="revokeEnrollmentToken(token)">撤销</NButton></td>
            </tr>
            <tr v-if="!enrollmentTokens.length"><td colspan="7" class="text-center">暂无令牌</td></tr>
          </tbody>
        </NTable>
      </NSpin>
    </NModal>

    <NModal v-model:show="deleteVisible" preset="card" title="删除设备管理记录" class="max-w-95vw w-600px" :mask-closable="!deleteSaving" :closable="!deleteSaving">
      <NAlert type="warning" :show-icon="false">
        删除不可通过此页面恢复，只清理管理记录及凭据，不是永久封禁；客户端以后可能重新注册。
        本功能创建的地址簿条目会移除，原有个人条目解除管理并保留，客户端本地缓存不会立即清除。
      </NAlert>
      <p class="my-4">请输入 RustDesk ID：{{ deleteForm.rustdesk_id }}</p>
      <NInput v-model:value="deleteForm.confirmation" placeholder="输入完整 RustDesk ID 确认" :disabled="deleteSaving" />
      <template #footer><NSpace justify="end">
        <NButton :disabled="deleteSaving" @click="deleteVisible = false">取消</NButton>
        <NButton type="error" :loading="deleteSaving" :disabled="deleteForm.confirmation !== deleteForm.rustdesk_id" @click="removeDevice">确认删除</NButton>
      </NSpace></template>
    </NModal>

    <NModal v-model:show="aliasVisible" preset="card" title="编辑设备别名" class="max-w-95vw w-600px" :mask-closable="!aliasSaving" :closable="!aliasSaving">
      <NForm label-placement="top">
        <NFormItem label="RustDesk ID"><NInput :value="aliasForm.rustdesk_id" disabled /></NFormItem>
        <NFormItem label="设备别名（最多 128 字，留空清除）"><NInput v-model:value="aliasForm.alias" :disabled="aliasSaving" /></NFormItem>
        <NFormItem label="发布到账号的个人地址簿">
          <NSelect v-model:value="aliasForm.address_book_ids" multiple clearable filterable :options="aliasOptions" :disabled="aliasSaving" />
        </NFormItem>
      </NForm>
      <NAlert type="info" :show-icon="false">
        请先用目标账号登录官方客户端创建个人地址簿。仅所选账号会收到条目；名称以 Web 为准，不改变实际 ID。
        取消目标会删除本功能新建的条目，原有个人条目保留并解除名称管理。官方客户端刷新后搜索别名、选择实际 ID 连接，不支持直接输入别名直连。
      </NAlert>
      <template #footer><NSpace justify="end">
        <NButton :disabled="aliasSaving" @click="aliasVisible = false">取消</NButton>
        <NButton type="primary" :loading="aliasSaving" @click="saveAlias">保存</NButton>
      </NSpace></template>
    </NModal>

    <NModal v-model:show="previewVisible" preset="card" title="当前生效配置" class="max-w-95vw w-600px">
      <NDescriptions v-if="preview" label-placement="left" bordered :column="1">
        <NDescriptionsItem label="版本">{{ preview.revision }}</NDescriptionsItem>
        <NDescriptionsItem label="设备组来源">{{ preview.group_source || 'none' }}</NDescriptionsItem>
        <NDescriptionsItem label="生效设备组">{{ preview.group_name || '无管理策略' }}</NDescriptionsItem>
        <NDescriptionsItem v-if="preview.group_warning" label="回退原因">{{ preview.group_warning }}</NDescriptionsItem>
        <NDescriptionsItem label="配置名称">{{ preview.profile_name || '公共服务' }}</NDescriptionsItem>
        <NDescriptionsItem label="API Server">{{ currentApiServer }}（固定）</NDescriptionsItem>
        <NDescriptionsItem label="ID Server">{{ preview.id_server }}</NDescriptionsItem>
        <NDescriptionsItem label="Relay Server">{{ preview.relay_server }}</NDescriptionsItem>
        <NDescriptionsItem label="Server Key">{{ preview.server_key }}</NDescriptionsItem>
        <NDescriptionsItem label="永久密码">{{ preview.password_set ? '已设置' : '未设置' }}</NDescriptionsItem>
        <NDescriptionsItem label="无人值守">
          {{ preview.unattended_enabled ? '开启' : '关闭' }} / {{ preview.root_command }}
        </NDescriptionsItem>
      </NDescriptions>
    </NModal>
  </div>
</template>

<style scoped>
.device-table :deep(.n-data-table-td) {
  vertical-align: top;
}
</style>
