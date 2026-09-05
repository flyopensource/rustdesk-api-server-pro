<script setup lang="tsx">
import { reactive, ref } from 'vue';
import { NButton, NFlex, NSelect, NSwitch, NTag } from 'naive-ui';
import { fetchDevicesList, updateDeviceProfile, updateDeviceUnattended } from '@/service/api/devices';
import { $t } from '@/locales';
import { useAppStore } from '@/store/modules/app';
import { useTable } from '@/hooks/common/table';
import TableHeader from './components/table-header.vue';
import AuditBaseLogsSearch from './components/search.vue';

const appStore = useAppStore();
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
        </NFlex>
      )
    }
  ]
});
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <AuditBaseLogsSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />

    <NCard :title="$t('route.devices')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
      <template #header-extra>
        <TableHeader v-model:columns="columnChecks" :loading="loading" @refresh="getData" />
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
  </div>
</template>

<style scoped></style>
