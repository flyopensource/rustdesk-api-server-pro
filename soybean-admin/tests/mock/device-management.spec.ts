import { expect, test } from '@playwright/test';

test('device alias, cancellation, state and deletion with mocked APIs only', async ({ page }) => {
  const device = {
    id: 1, rustdesk_id: '123456789', alias: '旧名称', hostname: 'mock-board', username: 'mock',
    disabled: false, is_online: false, group_id: 0, effective_group_id: 0, effective_group_name: '',
    group_source: 'none', group_warning: '', profile_name: '', profile_id_server: '', profile_password_set: false,
    all_files_access_ready: false, unattended_enabled: false, root_command: 'auto'
  };
  const group = {
    id: 2, name: 'android-board', enabled: true, is_default: true, member_count: 0, default_coverage_count: 1,
    profile_id: 1, profile_name: 'cn-jun', profile_enabled: true, unattended_enabled: true,
    password_set: true, root_command: 'auto', configuration_complete: true
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
    else if (path === '/devices/groups' && method === 'GET') data = {
      default_group_id: group.is_default ? group.id : 0, unassigned_device_count: 1, groups: [group]
    };
    else if (path === '/devices/groups' && method === 'PUT') {
      const body = route.request().postDataJSON();
      writes.push({ path, body });
      Object.assign(group, body);
    } else if (path === '/devices/server-profiles') {
      data = { profiles: [{ id: 1, name: 'cn-jun', id_server: 'id.example.com', relay_server: 'relay.example.com', server_key: 'key', enabled: true, group_count: 1 }] };
    } else if (path === '/devices/alias' && method === 'GET') {
      data = {
        alias: device.alias, address_book_ids: [10], targets: [{ id: 10, user_id: 2, username: 'alice', name: 'My address book', enabled: true }]
      };
    } else if (path === '/devices/alias' || path === '/devices/enabled' || path === '/devices/record') {
      const body = route.request().postDataJSON();
      writes.push({ path, body });
      if (path === '/devices/alias') device.alias = body.alias.trim();
      if (path === '/devices/enabled') device.disabled = !body.enabled;
      if (path === '/devices/record') removed = true;
    } else throw new Error(`Unexpected API request: ${method} ${path}`);
    await route.fulfill({ json: { code: 200, message: 'ok', data } });
  });
  await page.goto('/#/devices');
  await expect(page.getByRole('button', { name: '编辑别名', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '编辑别名', exact: true }).click();
  const aliasModal = page.locator('.n-modal').filter({ hasText: '编辑设备别名' });
  await expect(aliasModal).toBeVisible();
  await aliasModal.locator('input').nth(1).fill('取消的名称');
  await aliasModal.getByRole('button', { name: '取消', exact: true }).click();
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: '编辑别名', exact: true }).click();
  await aliasModal.locator('input').nth(1).fill('上海店一号机');
  await aliasModal.getByRole('button', { name: '保存', exact: true }).click();
  await expect(page.getByText('上海店一号机', { exact: true })).toBeVisible();
  expect(writes[0]).toEqual({ path: '/devices/alias', body: { id: 1, alias: '上海店一号机', address_book_ids: [10] } });
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
  await expect(page.getByRole('button', { name: '编辑别名', exact: true })).toHaveCount(0);
  expect(writes.at(-1)).toEqual({ path: '/devices/record', body: { id: 1, rustdesk_id: device.rustdesk_id } });

  await page.getByRole('button', { name: '设备组', exact: true }).click();
  const groupModal = page.locator('.n-modal').filter({ hasText: '设备组管理' });
  await expect(groupModal).toBeVisible();
  await expect(groupModal.getByText('固定密码', { exact: true })).toBeVisible();
  await expect(groupModal.getByText('Root 执行器', { exact: true })).toBeVisible();
  await expect(groupModal.getByText('auto 会依次探测 su 和 testsu')).toBeVisible();
  await groupModal.getByRole('button', { name: '编辑', exact: true }).click();
  const defaultSwitch = groupModal.locator('.n-form-item').filter({ hasText: '默认设备组' }).locator('.n-switch');
  await expect(defaultSwitch).toHaveClass(/n-switch--active/);
  await defaultSwitch.click();
  await groupModal.getByRole('button', { name: '保存设备组', exact: true }).click();
  expect(writes.find(item => item.path === '/devices/groups')?.body).toMatchObject({ id: 2, is_default: false });
});
