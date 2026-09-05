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
