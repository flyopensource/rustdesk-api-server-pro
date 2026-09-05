<script setup lang="tsx">
import { onMounted, reactive, ref } from 'vue';
import { NButton, NFlex, NSelect, NSwitch, NTag } from 'naive-ui';
import {
  createDeviceGroup,
  deleteDeviceGroup,
  deleteManagedDevicePolicy,
  fetchDeviceGroups,
  fetchDevicesList,
  fetchEffectiveDevicePolicy,
  fetchManagedDevicePolicy,
  updateDeviceGroup,
  updateDeviceGroupMembers,
  updateDeviceProfile,
  updateDeviceUnattended,
  updateManagedDevicePolicy
} from '@/service/api/devices';
import { $t } from '@/locales';
import { useAppStore } from '@/store/modules/app';
import { useTable } from '@/hooks/common/table';
import TableHeader from './components/table-header.vue';
import AuditBaseLogsSearch from './components/search.vue';

const appStore = useAppStore();
const groupsVisible = ref(false);
const groups = ref<Api.Devices.DeviceGroup[]>([]);
const groupForm = reactive({ id: 0, name: '', priority: 0, enabled: true, member_ids: '' });
const policyVisible = ref(false);
const policy = reactive({
  scope_type: 'global' as Api.Devices.PolicyScope,
  scope_id: 0,
  enabled: true,
  unattended_enabled: 'inherit' as 'inherit' | 'true' | 'false',
  root_command: '' as Api.Devices.PolicyDocument['unattended']['root_command'] | '',
  profile_enabled: 'inherit' as 'inherit' | 'true' | 'false',
  id_server: null as string | null,
  relay_server: null as string | null,
  api_server: null as string | null,
  key: '',
  permanent_password: '',
  key_set: false,
  password_set: false
});
const previewVisible = ref(false);
const preview = ref<Api.Devices.PolicyPreview | null>(null);
const profileVisible = ref(false);
const profile = reactive({
  id: 0,
  enabled: false,
  id_server: '',
  relay_server: '',
  api_server: '',
  key: '',
  permanent_password: '',
  key_set: false,
  password_set: false
});

function editProfile(row: Api.Devices.Device) {
  Object.assign(profile, {
    id: row.id,
    enabled: row.profile_enabled,
    id_server: row.profile_id_server || '',
    relay_server: row.profile_relay_server || '',
    api_server: row.profile_api_server || '',
    key: '',
    permanent_password: '',
    key_set: row.profile_key_set,
    password_set: row.profile_password_set
  });
  profileVisible.value = true;
}

async function loadGroups() {
  const { data: result } = await fetchDeviceGroups();
  groups.value = result || [];
}

function newGroup() {
  Object.assign(groupForm, { id: 0, name: '', priority: 0, enabled: true, member_ids: '' });
}

function editGroup(group: Api.Devices.DeviceGroup) {
  Object.assign(groupForm, { id: group.id, name: group.name, priority: group.priority, enabled: group.enabled, member_ids: group.device_ids.join(',') });
}

async function saveGroup() {
  const base = { name: groupForm.name.trim(), priority: groupForm.priority, enabled: groupForm.enabled };
  const response = groupForm.id ? await updateDeviceGroup({ id: groupForm.id, ...base }) : await createDeviceGroup(base);
  if (response.error) return;
  const groupId = groupForm.id || response.data?.id || 0;
  const deviceIds = groupForm.member_ids.split(',').map(value => Number(value.trim())).filter(value => Number.isInteger(value) && value > 0);
  if (groupId && (await updateDeviceGroupMembers(groupId, [...new Set(deviceIds)])).error) return;
  window.$message?.success('设备组已保存');
  newGroup();
  await loadGroups();
}

async function removeGroup(id: number) {
  if (!(await deleteDeviceGroup(id)).error) {
    window.$message?.success('设备组已删除');
    await loadGroups();
  }
}

async function editManagedPolicy(scopeType: Api.Devices.PolicyScope, scopeId: number) {
  Object.assign(policy, {
    scope_type: scopeType, scope_id: scopeId, enabled: true, unattended_enabled: 'inherit', root_command: '',
    profile_enabled: 'inherit', id_server: null, relay_server: null, api_server: null, key: '', permanent_password: '', key_set: false, password_set: false
  });
  const { data: existing } = await fetchManagedDevicePolicy(scopeType, scopeId);
  if (existing) {
    const document = existing.document;
    Object.assign(policy, {
      enabled: existing.enabled,
      unattended_enabled: document.unattended.enabled === undefined ? 'inherit' : String(document.unattended.enabled),
      root_command: document.unattended.root_command ?? '',
      profile_enabled: document.server_profile.enabled === undefined ? 'inherit' : String(document.server_profile.enabled),
      id_server: document.server_profile.id_server ?? null,
      relay_server: document.server_profile.relay_server ?? null,
      api_server: document.server_profile.api_server ?? null,
      key_set: Boolean(document.server_profile.key_set),
      password_set: Boolean(document.server_profile.permanent_password_set)
    });
  }
  policyVisible.value = true;
}

async function saveManagedPolicy() {
  const serverProfile: Api.Devices.PolicyDocument['server_profile'] = {};
  if (policy.profile_enabled !== 'inherit') serverProfile.enabled = policy.profile_enabled === 'true';
  if (policy.id_server !== null) serverProfile.id_server = policy.id_server.trim();
  if (policy.relay_server !== null) serverProfile.relay_server = policy.relay_server.trim();
  if (policy.api_server !== null) serverProfile.api_server = policy.api_server.trim();
  if (policy.key) serverProfile.key = policy.key;
  if (policy.permanent_password) serverProfile.permanent_password = policy.permanent_password;
  const unattended: Api.Devices.PolicyDocument['unattended'] = {};
  if (policy.unattended_enabled !== 'inherit') unattended.enabled = policy.unattended_enabled === 'true';
  if (policy.root_command) unattended.root_command = policy.root_command;
  const { error } = await updateManagedDevicePolicy({
    scope_type: policy.scope_type, scope_id: policy.scope_id, enabled: policy.enabled,
    document: { unattended, server_profile: serverProfile }
  });
  if (!error) {
    policyVisible.value = false;
    window.$message?.success('分层策略已发布');
    await getData();
  }
}

async function removeManagedPolicy() {
  const { error } = await deleteManagedDevicePolicy(policy.scope_type, policy.scope_id);
  if (!error) { policyVisible.value = false; window.$message?.success('覆盖策略已取消'); await getData(); }
}

async function showPreview(row: Api.Devices.Device) {
  const { data: result } = await fetchEffectiveDevicePolicy(row.id!);
  if (result) { preview.value = result; previewVisible.value = true; }
}

async function saveProfile() {
  const payload: Parameters<typeof updateDeviceProfile>[0] = {
    id: profile.id,
    enabled: profile.enabled,
    id_server: profile.id_server.trim(),
    relay_server: profile.relay_server.trim(),
    api_server: profile.api_server.trim()
  };
  if (profile.key) payload.key = profile.key;
  if (profile.permanent_password) payload.permanent_password = profile.permanent_password;
  const { error } = await updateDeviceProfile(payload);
  if (!error) {
    profileVisible.value = false;
    window.$message?.success('隐藏服务器配置已更新');
    await getData();
  }
}

async function saveUnattended(row: Api.Devices.Device, enabled = row.unattended_enabled, rootCommand = row.root_command) {
  const { error } = await updateDeviceUnattended({ id: row.id!, enabled, root_command: rootCommand });
  if (!error) {
    window.$message?.success('设备策略已更新');
    await getData();
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
  apiParams: {
    current: 1,
    size: 10,
    // if you want to use the searchParams in Form, you need to define the following properties, and the value is null
    // the value can not be undefined, otherwise the property in Form will not be reactive
    hostname: null,
    username: null,
    rustdesk_id: null
  },
  columns: () => [
    {
      key: 'id',
      title: 'ID',
      align: 'center'
    },
    {
      key: 'rustdesk_id',
      title: $t('dataMap.device.rustdesk_id'),
      align: 'center'
    },
    {
      key: 'hostname',
      title: $t('dataMap.device.hostname'),
      align: 'center'
    },
    {
      key: 'username',
      title: $t('dataMap.device.username'),
      align: 'center'
    },
    {
      key: 'version',
      title: $t('dataMap.device.version'),
      align: 'center'
    },
    {
      key: 'os',
      title: $t('dataMap.device.os'),
      align: 'center'
    },
    {
      key: 'memory',
      title: $t('dataMap.device.memory'),
      align: 'center'
    },
    {
      key: 'created_at',
      title: $t('dataMap.audit.created_at'),
      align: 'center'
    },
    {
      key: 'unattended_enabled',
      title: '无人值守',
      align: 'center',
      render: row => (
        <NSwitch
          value={row.unattended_enabled}
          onUpdateValue={value => saveUnattended(row, value, row.root_command || 'auto')}
        />
      )
    },
    {
      key: 'root_command',
      title: 'Root方式',
      align: 'center',
      render: row => (
        <NSelect
          class="w-110px"
          value={row.root_command || 'auto'}
          options={[
            { label: '自动', value: 'auto' },
            { label: 'su', value: 'su' },
            { label: 'testsu', value: 'testsu' },
            { label: '禁用', value: 'disabled' }
          ]}
          onUpdateValue={value => saveUnattended(row, row.unattended_enabled, value)}
        />
      )
    },
    {
      key: 'unattended_status',
      title: '生效状态',
      align: 'center',
      render: row => `${row.unattended_status || '未上报'} (${row.applied_revision || 0}/${row.policy_revision || 0})`
    },
    {
      key: 'service_running',
      title: '实际能力',
      align: 'center',
      render: row => `Root:${row.root_available ? row.root_executor || '是' : '否'} 录屏:${row.screen_capture_ready ? '是' : '否'} 无障碍:${row.accessibility_ready ? '是' : '否'} 服务:${row.service_running ? '运行' : '停止'}`
    },
    {
      key: 'unattended_error',
      title: '错误',
      align: 'center'
    },
    {
      key: 'profile_enabled',
      title: '隐藏服务器',
      align: 'center',
      render: row => (
        <NFlex vertical size={4} align="center">
          <NButton size="small" onClick={() => editProfile(row)}>{row.profile_enabled ? '已启用' : '配置'}</NButton>
          <NTag size="small" type={row.profile_connected ? 'success' : 'default'}>
            {row.profile_active_source || '未回报'} / {row.profile_connected ? '已连接' : '未连接'}
          </NTag>
          <span class="text-12px">{row.profile_applied_revision || 0}/{row.policy_revision}</span>
          <NFlex size={4}>
            <NButton size="tiny" onClick={() => editManagedPolicy('device', row.id!)}>覆盖</NButton>
            <NButton size="tiny" onClick={() => showPreview(row)}>预览</NButton>
          </NFlex>
        </NFlex>
      )
    }
  ]
});

onMounted(loadGroups);
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <AuditBaseLogsSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />

    <NCard :title="$t('route.devices')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
      <template #header-extra>
        <NFlex>
          <NButton size="small" @click="editManagedPolicy('global', 0)">全局策略</NButton>
          <NButton size="small" @click="groupsVisible = true">设备组</NButton>
          <TableHeader v-model:columns="columnChecks" :loading="loading" @refresh="getData" />
        </NFlex>
      </template>
      <NDataTable
        :columns="columns"
        :data="data"
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="962"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
    </NCard>

    <NModal v-model:show="profileVisible" preset="card" title="设备隐藏服务器配置" class="w-600px max-w-90vw">
      <NForm label-placement="left" label-width="110">
        <NFormItem label="启用"><NSwitch v-model:value="profile.enabled" /></NFormItem>
        <NFormItem label="ID Server"><NInput v-model:value="profile.id_server" /></NFormItem>
        <NFormItem label="Relay Server"><NInput v-model:value="profile.relay_server" /></NFormItem>
        <NFormItem label="API Server"><NInput v-model:value="profile.api_server" /></NFormItem>
        <NFormItem label="Server Key">
          <NInput v-model:value="profile.key" type="password" :placeholder="profile.key_set ? '已设置，留空保持不变' : '未设置'" />
        </NFormItem>
        <NFormItem label="永久密码">
          <NInput v-model:value="profile.permanent_password" type="password" :placeholder="profile.password_set ? '已设置，留空保持不变' : '未设置'" />
        </NFormItem>
      </NForm>
      <template #footer><div class="flex justify-end gap-12px"><NButton @click="profileVisible = false">取消</NButton><NButton type="primary" @click="saveProfile">保存并发布</NButton></div></template>
    </NModal>

    <NModal v-model:show="groupsVisible" preset="card" title="设备组管理" class="w-760px max-w-95vw">
      <NForm label-placement="left" label-width="90">
        <NFormItem label="组名"><NInput v-model:value="groupForm.name" /></NFormItem>
        <NFormItem label="优先级"><NInputNumber v-model:value="groupForm.priority" /></NFormItem>
        <NFormItem label="启用"><NSwitch v-model:value="groupForm.enabled" /></NFormItem>
        <NFormItem label="设备 ID"><NInput v-model:value="groupForm.member_ids" placeholder="后台设备数字 ID，逗号分隔" /></NFormItem>
        <NFlex justify="end"><NButton @click="newGroup">清空</NButton><NButton type="primary" @click="saveGroup">保存</NButton></NFlex>
      </NForm>
      <NDivider />
      <NList bordered>
        <NListItem v-for="group in groups" :key="group.id">
          {{ group.name }} · 优先级 {{ group.priority }} · {{ group.enabled ? '启用' : '停用' }} · {{ group.member_count }} 台
          <template #suffix><NFlex><NButton size="small" @click="editManagedPolicy('group', group.id)">策略</NButton><NButton size="small" @click="editGroup(group)">编辑</NButton><NButton size="small" type="error" @click="removeGroup(group.id)">删除</NButton></NFlex></template>
        </NListItem>
      </NList>
    </NModal>

    <NModal v-model:show="policyVisible" preset="card" :title="`分层策略：${policy.scope_type}:${policy.scope_id}`" class="w-650px max-w-95vw">
      <NForm label-placement="left" label-width="130">
        <NFormItem label="策略启用"><NSwitch v-model:value="policy.enabled" /></NFormItem>
        <NFormItem label="无人值守">
          <NSelect v-model:value="policy.unattended_enabled" :options="[{label:'继承',value:'inherit'},{label:'开启',value:'true'},{label:'关闭',value:'false'}]" />
        </NFormItem>
        <NFormItem label="Root 方式">
          <NSelect v-model:value="policy.root_command" :options="[{label:'继承',value:''},{label:'自动',value:'auto'},{label:'su',value:'su'},{label:'testsu',value:'testsu'},{label:'禁用',value:'disabled'}]" />
        </NFormItem>
        <NFormItem label="隐藏服务器">
          <NSelect v-model:value="policy.profile_enabled" :options="[{label:'继承',value:'inherit'},{label:'开启',value:'true'},{label:'关闭',value:'false'}]" />
        </NFormItem>
        <NFormItem label="ID Server"><NInput v-model:value="policy.id_server" clearable placeholder="留空值表示显式清空，清除控件表示继承" /></NFormItem>
        <NFormItem label="Relay Server"><NInput v-model:value="policy.relay_server" clearable /></NFormItem>
        <NFormItem label="API Server"><NInput v-model:value="policy.api_server" clearable /></NFormItem>
        <NFormItem label="Server Key"><NInput v-model:value="policy.key" type="password" :placeholder="policy.key_set ? '已设置，留空保持' : '留空继承'" /></NFormItem>
        <NFormItem label="永久密码"><NInput v-model:value="policy.permanent_password" type="password" :placeholder="policy.password_set ? '已设置，留空保持' : '留空继承'" /></NFormItem>
      </NForm>
      <template #footer><NFlex justify="space-between"><NButton type="error" @click="removeManagedPolicy">取消此层覆盖</NButton><NFlex><NButton @click="policyVisible = false">关闭</NButton><NButton type="primary" @click="saveManagedPolicy">保存并发布</NButton></NFlex></NFlex></template>
    </NModal>

    <NModal v-model:show="previewVisible" preset="card" title="有效策略预览" class="w-600px max-w-95vw">
      <NDescriptions v-if="preview" label-placement="left" bordered :column="1">
        <NDescriptionsItem label="合并层级">{{ preview.layers.join(' → ') }}</NDescriptionsItem>
        <NDescriptionsItem label="无人值守">{{ preview.effective.unattended_enabled ? '开启' : '关闭' }} / {{ preview.effective.root_command }}</NDescriptionsItem>
        <NDescriptionsItem label="隐藏服务器">{{ preview.effective.profile_enabled ? '开启' : '关闭' }}</NDescriptionsItem>
        <NDescriptionsItem label="ID Server">{{ preview.effective.id_server }}</NDescriptionsItem>
        <NDescriptionsItem label="Relay Server">{{ preview.effective.relay_server }}</NDescriptionsItem>
        <NDescriptionsItem label="API Server">{{ preview.effective.api_server }}</NDescriptionsItem>
        <NDescriptionsItem label="敏感字段">Key: {{ preview.effective.key_set ? '已设置' : '未设置' }} / 密码: {{ preview.effective.permanent_password_set ? '已设置' : '未设置' }}</NDescriptionsItem>
      </NDescriptions>
    </NModal>
  </div>
</template>

<style scoped></style>
