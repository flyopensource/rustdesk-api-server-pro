import { expect, test } from '@playwright/test';

test('managed connection ID, device name, state and deletion with mocked APIs only', async ({ page }) => {
  const device = {
    id: 1, rustdesk_id: '123456789', alias: '旧名称', hostname: 'mock-board', username: 'mock',
    disabled: false, is_online: false, group_id: 0, profile_assignment_id: 0,
    managed: true, requested_rustdesk_id: '', connection_id_status: 'unassigned', connection_id_error: '',
    all_files_access_ready: false, unattended_enabled: false, root_command: 'auto'
  };
  const writes: { path: string; body: any }[] = [];
  let removed = false;
  await page.addInitScript(() => {
    localStorage.setItem('SOY_token', JSON.stringify('mock-admin-token'));
    localStorage.setItem('SOY_lang', JSON.stringify('zh-CN'));
  });
  await page.route('**/proxy-default/**', async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace('/proxy-default', '');
    const method = route.request().method();
    let data: unknown = {};
    if (path === '/userinfo') data = { userId: '1', userName: 'mock-admin', roles: ['R_SUPER'], buttons: [] };
    else if (path === '/devices/list') data = { current: 1, size: 10, total: removed ? 0 : 1, records: removed ? [] : [device] };
    else if (path === '/devices/groups') data = [];
    else if (path === '/devices/server-profiles') data = { global_profile_id: 0, profiles: [] };
    else if (path === '/devices/alias' && method === 'GET') data = {
      alias: device.alias, address_book_ids: [10], targets: [{ id: 10, user_id: 2, username: 'alice', name: 'My address book', enabled: true }]
    };
    else if (path === '/devices/alias' || path === '/devices/connection-id' || path === '/devices/enabled' || path === '/devices/record') {
      const body = route.request().postDataJSON();
      writes.push({ path, body });
      if (path === '/devices/alias') device.alias = body.alias.trim();
      if (path === '/devices/connection-id') {
        device.requested_rustdesk_id = body.connection_id;
        device.connection_id_status = 'pending';
      }
      if (path === '/devices/enabled') device.disabled = !body.enabled;
      if (path === '/devices/record') removed = true;
    } else throw new Error(`Unexpected API request: ${method} ${path}`);
    await route.fulfill({ json: { code: 200, message: 'ok', data } });
  });
  await page.goto('/#/devices');
  await page.getByRole('button', { name: '设置连接 ID', exact: true }).click();
  const connectionIdModal = page.locator('.n-modal').filter({ hasText: '设置连接 ID' });
  const submitConnectionId = connectionIdModal.getByRole('button', { name: '下发连接 ID', exact: true });
  await connectionIdModal.getByPlaceholder('例如 shop23-a01').fill('bad');
  await expect(submitConnectionId).toBeDisabled();
  await connectionIdModal.getByPlaceholder('例如 shop23-a01').fill('Shop23-A01');
  await submitConnectionId.click();
  await expect(page.getByText('目标：shop23-a01', { exact: true })).toBeVisible();
  await expect(page.getByText('等待设备应用', { exact: true })).toBeVisible();
  expect(writes[0]).toEqual({ path: '/devices/connection-id', body: { id: 1, connection_id: 'shop23-a01' } });
  await expect(page.getByRole('button', { name: '编辑名称', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '编辑名称', exact: true }).click();
  const aliasModal = page.locator('.n-modal').filter({ hasText: '编辑设备名称' });
  await expect(aliasModal).toBeVisible();
  await aliasModal.locator('input').nth(1).fill('取消的名称');
  await aliasModal.getByRole('button', { name: '取消', exact: true }).click();
  expect(writes).toHaveLength(1);
  await page.getByRole('button', { name: '编辑名称', exact: true }).click();
  await aliasModal.locator('input').nth(1).fill('上海店一号机');
  await aliasModal.getByRole('button', { name: '保存', exact: true }).click();
  await expect(page.getByText('上海店一号机', { exact: true })).toBeVisible();
  expect(writes[1]).toEqual({ path: '/devices/alias', body: { id: 1, alias: '上海店一号机', address_book_ids: [10] } });
  await page.getByRole('button', { name: '停用', exact: true }).click();
  await page.getByRole('button', { name: '确认', exact: true }).click();
  await expect(page.getByRole('button', { name: '重新启用', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '删除记录', exact: true }).click();
  const deleteModal = page.locator('.n-modal').filter({ hasText: '删除设备管理记录' });
  const confirm = deleteModal.getByRole('button', { name: '确认删除', exact: true });
  await expect(confirm).toBeDisabled();
  await deleteModal.getByPlaceholder('输入完整 RustDesk ID 确认').fill('wrong-id');
  await expect(confirm).toBeDisabled();
  await deleteModal.getByPlaceholder('输入完整 RustDesk ID 确认').fill(device.rustdesk_id);
  await confirm.click();
  await expect(page.getByRole('button', { name: '编辑名称', exact: true })).toHaveCount(0);
  expect(writes.at(-1)).toEqual({ path: '/devices/record', body: { id: 1, rustdesk_id: device.rustdesk_id } });
});
