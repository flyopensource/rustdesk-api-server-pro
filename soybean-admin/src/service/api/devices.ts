import { request } from '../request';

export function fetchDevicesList(params: any) {
  return request<Api.Devices.DevicesList>({ url: '/devices/list', params });
}

export function updateDeviceUnattended(data: { id: number; enabled: boolean; root_command: string }) {
  return request<{ policy_revision: number }>({ url: '/devices/unattended', method: 'put', data });
}
