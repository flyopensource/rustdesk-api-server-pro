import { request } from '../request';

export function fetchDevicesList(params: any) {
  return request<Api.Devices.DevicesList>({ url: '/devices/list', params });
}

export function updateDeviceUnattended(data: { id: number; enabled: boolean; root_command: string }) {
  return request<{ policy_revision: number }>({ url: '/devices/unattended', method: 'put', data });
}

export function fetchDeviceGroups() {
  return request<Api.Devices.DeviceGroup[]>({ url: '/devices/groups' });
}

export function createDeviceGroup(data: Pick<Api.Devices.DeviceGroup, 'name' | 'enabled'>) {
  return request<{ id: number }>({ url: '/devices/groups', method: 'post', data });
}

export function updateDeviceGroup(data: Pick<Api.Devices.DeviceGroup, 'id' | 'name' | 'enabled'>) {
  return request({ url: '/devices/groups', method: 'put', data });
}

export function deleteDeviceGroup(id: number) {
  return request({ url: '/devices/groups', method: 'delete', params: { id } });
}

export function updateDeviceGroupMembers(group_id: number, device_ids: number[]) {
  return request({ url: '/devices/groups/members', method: 'put', data: { group_id, device_ids } });
}

export function fetchServerProfiles() {
  return request<Api.Devices.ServerProfilesResult>({ url: '/devices/server-profiles' });
}

export function createServerProfile(data: Api.Devices.ServerProfileInput) {
  return request<Api.Devices.ServerProfile>({ url: '/devices/server-profiles', method: 'post', data });
}

export function updateServerProfile(data: Api.Devices.ServerProfileInput & { id: number }) {
  return request<Api.Devices.ServerProfile>({ url: '/devices/server-profiles', method: 'put', data });
}

export function deleteServerProfile(id: number) {
  return request({ url: '/devices/server-profiles', method: 'delete', params: { id } });
}

export function updateServerProfileAssignment(
  scope_type: Api.Devices.StrategyScope,
  scope_id: number,
  profile_id: number
) {
  return request<{ revision: number }>({
    url: '/devices/server-profile-assignment',
    method: 'put',
    data: { scope_type, scope_id, profile_id }
  });
}

export function fetchServerProfilePreview(device_id: number) {
  return request<Api.Devices.ServerProfilePreview>({ url: '/devices/server-profile-preview', params: { device_id } });
}
