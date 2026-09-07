import { request } from '../request';

export function updateDeviceEnabled(id: number, enabled: boolean) {
  return request({ url: '/devices/enabled', method: 'put', data: { id, enabled } });
}

export function fetchDevicesList(params: any) {
  return request<Api.Devices.DevicesList>({ url: '/devices/list', params });
}

export function updateDeviceUnattended(data: { id: number; enabled: boolean; root_command: string }) {
  return request<{ policy_revision: number }>({ url: '/devices/unattended', method: 'put', data });
}

export function fetchDeviceGroups() {
  return request<Api.Devices.DeviceGroup[]>({ url: '/devices/groups' });
}

export function createDeviceGroup(data: Pick<Api.Devices.DeviceGroup, 'name' | 'enabled' | 'profile_id'>) {
  return request<{ id: number }>({ url: '/devices/groups', method: 'post', data });
}

export function updateDeviceGroup(data: Pick<Api.Devices.DeviceGroup, 'id' | 'name' | 'enabled' | 'profile_id'>) {
  return request({ url: '/devices/groups', method: 'put', data });
}

export function deleteDeviceGroup(id: number) {
  return request({ url: '/devices/groups', method: 'delete', params: { id } });
}

export function updateDeviceGroupAssignment(id: number, group_id: number) {
  return request<Api.Devices.ServerProfilePreview>({ url: '/devices/group', method: 'put', data: { id, group_id } });
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

export function updateDeviceServerProfile(id: number, profile_id: number) {
  return request<Api.Devices.ServerProfilePreview>({
    url: '/devices/server-profile',
    method: 'put',
    data: { id, profile_id }
  });
}

export function updateGlobalServerProfile(profile_id: number) {
  return request({ url: '/devices/global-server-profile', method: 'put', data: { profile_id } });
}

export function fetchServerProfilePreview(device_id: number) {
  return request<Api.Devices.ServerProfilePreview>({ url: '/devices/server-profile-preview', params: { device_id } });
}
