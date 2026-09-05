import { request } from '../request';

export function fetchDevicesList(params: any) {
  return request<Api.Devices.DevicesList>({ url: '/devices/list', params });
}

export function updateDeviceUnattended(data: { id: number; enabled: boolean; root_command: string }) {
  return request<{ policy_revision: number }>({ url: '/devices/unattended', method: 'put', data });
}

export function updateDeviceProfile(data: {
  id: number;
  enabled: boolean;
  id_server: string;
  relay_server: string;
  api_server: string;
  key?: string;
  permanent_password?: string;
}) {
  return request<{ policy_revision: number }>({ url: '/devices/profile', method: 'put', data });
}

export function fetchDeviceGroups() {
  return request<Api.Devices.DeviceGroup[]>({ url: '/devices/groups' });
}

export function createDeviceGroup(data: Omit<Api.Devices.DeviceGroup, 'id' | 'member_count' | 'device_ids'>) {
  return request<{ id: number }>({ url: '/devices/groups', method: 'post', data });
}

export function updateDeviceGroup(data: Pick<Api.Devices.DeviceGroup, 'id' | 'name' | 'priority' | 'enabled'>) {
  return request({ url: '/devices/groups', method: 'put', data });
}

export function deleteDeviceGroup(id: number) {
  return request({ url: '/devices/groups', method: 'delete', params: { id } });
}

export function updateDeviceGroupMembers(group_id: number, device_ids: number[]) {
  return request({ url: '/devices/groups/members', method: 'put', data: { group_id, device_ids } });
}

export function fetchManagedDevicePolicy(scope_type: Api.Devices.PolicyScope, scope_id: number) {
  return request<Api.Devices.ManagedPolicy | null>({ url: '/devices/policy', params: { scope_type, scope_id } });
}

export function updateManagedDevicePolicy(data: Api.Devices.ManagedPolicyInput) {
  return request<{ revision: number }>({ url: '/devices/policy', method: 'put', data });
}

export function deleteManagedDevicePolicy(scope_type: Api.Devices.PolicyScope, scope_id: number) {
  return request({ url: '/devices/policy', method: 'delete', params: { scope_type, scope_id } });
}

export function fetchEffectiveDevicePolicy(device_id: number) {
  return request<Api.Devices.PolicyPreview>({ url: '/devices/policy/preview', params: { device_id } });
}
