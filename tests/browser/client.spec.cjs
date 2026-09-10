const { test, expect } = require('@playwright/test');

async function api(page, endpoint, method = 'GET', data) {
  return page.evaluate(async ({ endpoint, method, data }) => {
    const response = await fetch(endpoint, {
      method,
      headers: { 'X-Elciber-Token': document.querySelector('meta[name="elciber-token"]').content, 'Content-Type': 'application/json' },
      body: data === undefined ? undefined : JSON.stringify(data),
    });
    return { status: response.status, data: await response.json() };
  }, { endpoint, method, data });
}
async function reset(page, base = '/') {
  await page.goto(base);
  await expect(page.locator('[data-open="create-dialog"]').first()).toBeEnabled();
  const state = await api(page, '/api/state');
  for (const room of state.data.rooms) await api(page, `/api/rooms/${room.id}`, 'DELETE', {});
  await page.reload();
  await expect(page.locator('[data-open="create-dialog"]').first()).toBeEnabled();
}
async function createRoom(page, name = 'La partida del viernes', game = 'Sesión cooperativa') {
  await page.getByRole('button', { name: 'Crear una sala', exact: true }).click();
  await page.locator('#create-name').fill(name);
  await page.locator('#create-game').fill(game);
  await page.locator('#create-form button[type="submit"]').click();
  await expect(page.locator('#create-dialog')).not.toBeVisible();
  await expect(page.locator('#room-title')).toHaveText(name);
}

test.beforeEach(async ({ page }) => { await reset(page); });

test('empty state, missing engine, safe defaults and zero third-party requests', async ({ page }) => {
  const external = [];
  page.on('request', r => { if (!r.url().startsWith('http://127.0.0.1:')) external.push(r.url()); });
  await page.reload();
  await expect(page.locator('#home-title')).toContainText('Jugar juntos.');
  await expect(page.locator('#engine-install')).toBeVisible();
  await expect(page.locator('#engine-mode')).toContainText(/inspección/i);
  const state = await api(page, '/api/state');
  expect(state.data.rooms).toEqual([]);
  expect(state.data.engine.mode).toBe('inspection');
  expect(state.data.engine.status).toBe('stopped');
  expect(external).toEqual([]);
});

test('create form retains edits and keyboard focus across polling, then persists', async ({ page }) => {
  await page.getByRole('button', { name: 'Crear una sala', exact: true }).click();
  await page.locator('#create-name').fill('La partida del viernes');
  await page.locator('#create-game').fill('Sesión cooperativa');
  await page.locator('#create-game').focus();
  // Deliberately cross a 3-second polling interval: this is the assertion under test.
  await page.waitForTimeout(3500);
  await expect(page.locator('#create-name')).toHaveValue('La partida del viernes');
  await expect(page.locator('#create-game')).toBeFocused();
  await page.locator('#create-form button[type="submit"]').click();
  await expect(page.locator('#room-title')).toHaveText('La partida del viernes');
  await page.reload();
  await expect(page.locator('#room-title')).toHaveText('La partida del viernes');
  const state = await api(page, '/api/state');
  expect(state.data.rooms).toHaveLength(1);
  expect(JSON.stringify(state.data)).not.toContain('secret');
});

test('real invitation imports into another isolated client, never autoconnects', async ({ page, browser }) => {
  await createRoom(page);
  await page.locator('#share-room').click();
  await expect(page.locator('#share-invite')).not.toHaveValue('');
  const invitation = await page.locator('#share-invite').inputValue();
  expect(invitation.startsWith('elciber1:')).toBe(true);
  await page.locator('#share-dialog [data-close]').first().click();
  await expect(page.locator('#share-invite')).toHaveValue('');
  const other = await browser.newContext();
  const otherPage = await other.newPage();
  try {
    await reset(otherPage, 'http://127.0.0.1:37965/');
    await otherPage.locator('[data-open="import-dialog"]').first().click();
    await otherPage.locator('#import-invite').fill(invitation);
    await otherPage.locator('#import-form button[type="submit"]').click();
    await expect(otherPage.locator('#room-title')).toHaveText('La partida del viernes');
    const state = await api(otherPage, '/api/state');
    expect(state.data.engine.status).toBe('stopped');
    expect(state.data.rooms).toHaveLength(1);
  } finally { await other.close(); }
  const storage = await page.evaluate(() => JSON.stringify(localStorage));
  expect(storage.includes(invitation)).toBe(false);
});

test('untrusted room names render as text; deletion is explicit', async ({ page }) => {
  const name = '<img src=x onerror=alert(1)>';
  await createRoom(page, name, 'Prueba de texto');
  expect(await page.locator('#room-title img').count()).toBe(0);
  await expect(page.locator('#connect-room')).toBeDisabled();
  await page.locator('#delete-room').click();
  await page.locator('#delete-dialog [data-close]').last().click();
  await expect(page.locator('#room-title')).toHaveText(name);
  await page.locator('#delete-room').click();
  await page.locator('#delete-form button[type="submit"]').click();
  await expect(page.locator('#home-view')).toBeVisible();
  expect((await api(page, '/api/state')).data.rooms).toEqual([]);
});

test('malformed invitation shows recoverable error without losing draft', async ({ page }) => {
  await page.locator('[data-open="import-dialog"]').first().click();
  await page.locator('#import-invite').fill('not-an-invitation');
  await page.locator('#import-form button[type="submit"]').click();
  await expect(page.locator('#import-form .form-error')).toBeVisible();
  await expect(page.locator('#import-invite')).toHaveValue('not-an-invitation');
  await expect(page.locator('#import-form button[type="submit"]')).toBeEnabled();
});

test('compact layout stays within viewport and dialog works with keyboard', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole('button', { name: 'Crear una sala', exact: true }).click();
  await expect(page.locator('#create-name')).toBeFocused();
  await page.locator('#create-name').fill('Equipo nocturno');
  await page.waitForTimeout(3500);
  const geometry = await page.evaluate(() => ({ width: innerWidth, body: document.documentElement.scrollWidth, dialog: document.querySelector('#create-dialog').getBoundingClientRect().toJSON() }));
  expect(geometry.body).toBeLessThanOrEqual(geometry.width);
  expect(geometry.dialog.x).toBeGreaterThanOrEqual(0);
  expect(geometry.dialog.right).toBeLessThanOrEqual(geometry.width);
  await page.keyboard.press('Escape');
  await expect(page.locator('#create-dialog')).not.toBeVisible();
});

test('real runtime screenshots without invitations or personal data', async ({ page }) => {
  await page.screenshot({ path: 'docs/images/01-inicio.png', fullPage: true, animations: 'disabled' });
  await createRoom(page);
  await expect(page.locator('#toast')).not.toBeVisible({ timeout: 9000 });
  await page.screenshot({ path: 'docs/images/02-sala.png', fullPage: true, animations: 'disabled' });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: 'docs/images/03-movil.png', fullPage: true, animations: 'disabled' });
});
