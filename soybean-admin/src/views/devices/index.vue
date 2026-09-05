<script setup lang="tsx">
import { fetchDevicesList, updateDeviceUnattended } from '@/service/api/devices';
import { $t } from '@/locales';
import { useAppStore } from '@/store/modules/app';
import { useTable } from '@/hooks/common/table';
import TableHeader from './components/table-header.vue';
import AuditBaseLogsSearch from './components/search.vue';

const appStore = useAppStore();

async function saveUnattended(row: Api.Devices.Device, enabled = row.unattended_enabled, rootCommand = row.root_command) {
  const { error } = await updateDeviceUnattended({ id: row.id, enabled, root_command: rootCommand });
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
      key: 'capabilities',
      title: '实际能力',
      align: 'center',
      render: row => `Root:${row.root_available ? row.root_executor || '是' : '否'} 录屏:${row.screen_capture_ready ? '是' : '否'} 无障碍:${row.accessibility_ready ? '是' : '否'} 服务:${row.service_running ? '运行' : '停止'}`
    },
    {
      key: 'unattended_error',
      title: '错误',
      align: 'center'
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
  </div>
</template>

<style scoped></style>
